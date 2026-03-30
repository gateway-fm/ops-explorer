package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"explorer/internal/log"
	"explorer/internal/verifier"

	"explorer/pkg/eth/common"
)

// etherscanVerifyResult stores the outcome of a verification triggered via the
// Etherscan-compatible API so that checkverifystatus can return it later.
type etherscanVerifyResult struct {
	Success bool
	Message string
}

// handleEtherscanRPC is the entry-point for the Etherscan-compatible
// module/action RPC used by `hardhat verify` and `forge verify-contract`.
func (s *Server) handleEtherscanRPC(w http.ResponseWriter, r *http.Request) {
	// Parse module/action from query string first, then from form values.
	module := r.URL.Query().Get("module")
	action := r.URL.Query().Get("action")

	if module == "" || action == "" {
		// Try parsing the body (multipart or form-encoded).
		_ = r.ParseMultipartForm(10 << 20)
		if module == "" {
			module = r.FormValue("module")
		}
		if action == "" {
			action = r.FormValue("action")
		}
	}

	if module == "" || action == "" {
		http.NotFound(w, r)
		return
	}

	module = strings.ToLower(module)
	action = strings.ToLower(action)

	switch module {
	case "contract":
		switch action {
		case "verifysourcecode":
			s.handleEtherscanVerify(w, r)
		case "checkverifystatus":
			s.handleEtherscanCheckStatus(w, r)
		case "getabi":
			s.handleEtherscanGetABI(w, r)
		case "getsourcecode":
			s.handleEtherscanGetSource(w, r)
		default:
			etherscanError(w, "Unknown action: "+action)
		}
	default:
		etherscanError(w, "Unknown module: "+module)
	}
}

// handleEtherscanVerify handles module=contract&action=verifysourcecode.
func (s *Server) handleEtherscanVerify(w http.ResponseWriter, r *http.Request) {
	if s.verifier == nil {
		etherscanError(w, "Contract verification not configured (solc not available)")
		return
	}

	// Hardhat and Foundry send form-encoded or multipart POST bodies.
	_ = r.ParseMultipartForm(10 << 20)

	contractAddress := r.FormValue("contractaddress")
	sourceCode := r.FormValue("sourceCode")
	codeFormat := r.FormValue("codeformat")
	contractName := r.FormValue("contractname")
	compilerVersion := r.FormValue("compilerversion")
	optimizationUsed := r.FormValue("optimizationUsed")
	runs := r.FormValue("runs")
	evmVersion := r.FormValue("evmversion")

	// Accept both the Etherscan typo and correct spelling.
	constructorArgs := r.FormValue("constructorArguements")
	if constructorArgs == "" {
		constructorArgs = r.FormValue("constructorArguments")
	}

	// Strip leading "0x" from constructor args if present.
	constructorArgs = strings.TrimPrefix(constructorArgs, "0x")

	// Normalise compiler version: tools send "v0.8.19+commit.xxx" but our
	// verifier expects just "0.8.19".
	compilerVersion = normaliseCompilerVersion(compilerVersion)

	// Collect libraries from numbered params (libraryname1..10, libraryaddress1..10).
	libs := map[string]string{}
	for i := 1; i <= 10; i++ {
		name := r.FormValue(fmt.Sprintf("libraryname%d", i))
		addr := r.FormValue(fmt.Sprintf("libraryaddress%d", i))
		if name != "" && addr != "" {
			libs[name] = addr
		}
	}

	// Generate a GUID for the async status check pattern.
	guid := fmt.Sprintf("%s%x", strings.ToLower(strings.TrimPrefix(contractAddress, "0x")), time.Now().UnixNano())

	var verifyResult *verifier.VerifyResponse
	var verifyErr error

	switch strings.ToLower(codeFormat) {
	case "solidity-standard-json-input":
		verifyResult, verifyErr = s.etherscanVerifyStandardJSON(r, contractAddress, sourceCode, contractName, compilerVersion, constructorArgs, libs)
	default:
		// solidity-single-file or empty (default)
		verifyResult, verifyErr = s.etherscanVerifySingleFile(r, contractAddress, sourceCode, contractName, compilerVersion, optimizationUsed, runs, evmVersion, constructorArgs, libs)
	}

	if verifyErr != nil {
		log.Error("etherscan verify internal error", "error", verifyErr)
		s.etherscanResults.Store(guid, &etherscanVerifyResult{Success: false, Message: verifyErr.Error()})
		etherscanError(w, verifyErr.Error())
		return
	}

	if verifyResult != nil && !verifyResult.Success {
		msg := verifyResult.Error
		if msg == "" {
			msg = "Unable to verify"
		}
		s.etherscanResults.Store(guid, &etherscanVerifyResult{Success: false, Message: msg})
		etherscanJSON(w, "0", "Fail - Unable to verify", msg)
		return
	}

	s.etherscanResults.Store(guid, &etherscanVerifyResult{Success: true, Message: "Pass - Verified"})
	etherscanJSON(w, "1", "OK", guid)
}

func (s *Server) etherscanVerifyStandardJSON(
	r *http.Request,
	address, sourceCode, contractName, compilerVersion, constructorArgs string,
	libs map[string]string,
) (*verifier.VerifyResponse, error) {
	// contractName may be "contracts/Token.sol:Token" — split to get file and name.
	contractFile := ""
	if idx := strings.LastIndex(contractName, ":"); idx >= 0 {
		contractFile = contractName[:idx]
		contractName = contractName[idx+1:]
	}

	// Inject libraries into the standard JSON input if any were provided.
	standardInput := json.RawMessage(sourceCode)
	if len(libs) > 0 {
		standardInput = injectLibraries(standardInput, libs)
	}

	req := &verifier.StandardJSONVerifyRequest{
		Address:         address,
		CompilerVersion: compilerVersion,
		ContractName:    contractName,
		ContractFile:    contractFile,
		StandardInput:   standardInput,
		ConstructorArgs: constructorArgs,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal standard JSON request: %w", err)
	}

	return s.verifier.VerifyStandardJSON(r.Context(), data)
}

func (s *Server) etherscanVerifySingleFile(
	r *http.Request,
	address, sourceCode, contractName, compilerVersion, optimizationUsed, runs, evmVersion, constructorArgs string,
	libs map[string]string,
) (*verifier.VerifyResponse, error) {
	optUsed := optimizationUsed == "1"

	optRuns := 200
	if runs != "" {
		if v, err := strconv.Atoi(runs); err == nil {
			optRuns = v
		}
	}

	filename := contractName + ".sol"

	req := &verifier.VerifyRequest{
		Address:          address,
		SourceFiles:      map[string]string{filename: sourceCode},
		MainContractFile: filename,
		ContractName:     contractName,
		CompilerVersion:  compilerVersion,
		OptimizationUsed: optUsed,
		OptimizationRuns: optRuns,
		EVMVersion:       evmVersion,
		ConstructorArgs:  constructorArgs,
		Libraries:        libs,
	}

	return s.verifier.Verify(r.Context(), req)
}

// handleEtherscanCheckStatus handles module=contract&action=checkverifystatus.
func (s *Server) handleEtherscanCheckStatus(w http.ResponseWriter, r *http.Request) {
	guid := r.URL.Query().Get("guid")
	if guid == "" {
		guid = r.FormValue("guid")
	}

	result, ok := s.etherscanResults.Load(guid)
	if !ok {
		// Hardhat retries this endpoint; "Pending in queue" tells it to keep polling.
		etherscanJSON(w, "0", "Pending in queue", "Pending in queue")
		return
	}

	res := result.(*etherscanVerifyResult)
	if res.Success {
		etherscanJSON(w, "1", "OK", "Pass - Verified")
	} else {
		etherscanJSON(w, "0", "Fail - Unable to verify", res.Message)
	}
}

// handleEtherscanGetABI handles module=contract&action=getabi.
func (s *Server) handleEtherscanGetABI(w http.ResponseWriter, r *http.Request) {
	address := r.URL.Query().Get("address")
	if address == "" {
		address = r.FormValue("address")
	}

	if !common.IsHexAddress(address) {
		etherscanError(w, "Invalid address format")
		return
	}
	address = common.HexToAddress(address).Hex()

	contract, err := s.provider.GetContract(r.Context(), address)
	if err != nil {
		etherscanError(w, "Error fetching contract: "+err.Error())
		return
	}
	if contract == nil || !contract.IsVerified || len(contract.ABI) == 0 {
		etherscanJSON(w, "0", "NOTOK", "Contract source code not verified")
		return
	}

	etherscanJSON(w, "1", "OK", string(contract.ABI))
}

// handleEtherscanGetSource handles module=contract&action=getsourcecode.
func (s *Server) handleEtherscanGetSource(w http.ResponseWriter, r *http.Request) {
	address := r.URL.Query().Get("address")
	if address == "" {
		address = r.FormValue("address")
	}

	if !common.IsHexAddress(address) {
		etherscanError(w, "Invalid address format")
		return
	}
	address = common.HexToAddress(address).Hex()

	contract, err := s.provider.GetContract(r.Context(), address)
	if err != nil {
		etherscanError(w, "Error fetching contract: "+err.Error())
		return
	}
	if contract == nil || !contract.IsVerified {
		etherscanJSON(w, "0", "NOTOK", "Contract source code not verified")
		return
	}

	sourceCode := ""
	if contract.SourceCode != nil {
		sourceCode = *contract.SourceCode
	}
	contractName := ""
	if contract.ContractName != nil {
		contractName = *contract.ContractName
	}
	compilerVersion := ""
	if contract.CompilerVersion != nil {
		compilerVersion = *contract.CompilerVersion
	}
	optimizationUsed := "0"
	if contract.OptimizationUsed != nil && *contract.OptimizationUsed {
		optimizationUsed = "1"
	}
	runs := 200
	if contract.OptimizationRuns != nil {
		runs = *contract.OptimizationRuns
	}
	evmVersion := ""
	if contract.EVMVersion != nil {
		evmVersion = *contract.EVMVersion
	}
	constructorArgs := ""
	if contract.ConstructorArgs != nil {
		constructorArgs = *contract.ConstructorArgs
	}
	licenseType := ""
	if contract.LicenseType != nil {
		licenseType = *contract.LicenseType
	}

	abiStr := "[]"
	if len(contract.ABI) > 0 {
		abiStr = string(contract.ABI)
	}

	// Etherscan returns an array of one element.
	result := []map[string]any{{
		"SourceCode":           sourceCode,
		"ABI":                  abiStr,
		"ContractName":         contractName,
		"CompilerVersion":      compilerVersion,
		"OptimizationUsed":     optimizationUsed,
		"Runs":                 fmt.Sprintf("%d", runs),
		"ConstructorArguments": constructorArgs,
		"EVMVersion":           evmVersion,
		"Library":              "",
		"LicenseType":          licenseType,
		"Proxy":                "0",
		"Implementation":       "",
		"SwarmSource":          "",
	}}

	etherscanJSON(w, "1", "OK", result)
}

// --- helpers ---

func etherscanJSON(w http.ResponseWriter, status, message string, result any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status":  status,
		"message": message,
		"result":  result,
	})
}

func etherscanError(w http.ResponseWriter, message string) {
	etherscanJSON(w, "0", "NOTOK", message)
}

// normaliseCompilerVersion converts "v0.8.19+commit.7dd6d404" to "0.8.19".
func normaliseCompilerVersion(v string) string {
	v = strings.TrimPrefix(v, "v")
	if idx := strings.Index(v, "+"); idx >= 0 {
		v = v[:idx]
	}
	return strings.TrimSpace(v)
}

// injectLibraries merges library addresses into a Solidity standard JSON input
// under settings.libraries. The libs map keys may be fully-qualified
// ("contracts/Lib.sol:MyLib") or bare names ("MyLib"). For bare names we place
// them under an empty-string file key, which solc accepts.
func injectLibraries(input json.RawMessage, libs map[string]string) json.RawMessage {
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(input, &parsed); err != nil {
		return input
	}

	var settings map[string]json.RawMessage
	if raw, ok := parsed["settings"]; ok {
		if err := json.Unmarshal(raw, &settings); err != nil {
			settings = map[string]json.RawMessage{}
		}
	} else {
		settings = map[string]json.RawMessage{}
	}

	// Build the libraries map: file -> name -> address
	libMap := map[string]map[string]string{}
	for name, addr := range libs {
		file := ""
		libName := name
		if idx := strings.LastIndex(name, ":"); idx >= 0 {
			file = name[:idx]
			libName = name[idx+1:]
		}
		if libMap[file] == nil {
			libMap[file] = map[string]string{}
		}
		libMap[file][libName] = addr
	}

	libBytes, err := json.Marshal(libMap)
	if err != nil {
		return input
	}
	settings["libraries"] = libBytes

	settingsBytes, err := json.Marshal(settings)
	if err != nil {
		return input
	}
	parsed["settings"] = settingsBytes

	out, err := json.Marshal(parsed)
	if err != nil {
		return input
	}
	return out
}
