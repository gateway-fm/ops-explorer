package publicapi

func buildOpenAPISpec() map[string]any {
	return map[string]any{
		"openapi": "3.0.3",
		"info": map[string]any{
			"title":       "Block Explorer Public API",
			"version":     "1.0.0",
			"description": "A read-only, rate-limited REST API for querying blockchain data from the block explorer.",
		},
		"servers": []map[string]any{
			{"url": "/api/v1", "description": "Public API v1"},
		},
		"paths":      buildPaths(),
		"components": buildComponents(),
	}
}

func buildPaths() map[string]any {
	return map[string]any{
		"/blocks": map[string]any{
			"get": endpoint("List blocks", "Returns a paginated list of blocks.", []param{
				queryParam("limit", "integer", "Number of blocks to return (max 100)", false),
				queryParam("before", "string", "Cursor: return blocks before this block number", false),
			}, ref("CursorPaginatedBlocks")),
		},
		"/blocks/latest": map[string]any{
			"get": endpoint("Get latest block", "Returns the most recent block.", nil, ref("Block")),
		},
		"/blocks/{number}": map[string]any{
			"get": endpoint("Get block by number", "Returns block detail with transactions.", []param{
				pathParam("number", "integer", "Block number"),
			}, map[string]any{
				"type": "object",
				"properties": map[string]any{
					"block":        ref("Block"),
					"transactions": map[string]any{"type": "array", "items": ref("Transaction")},
				},
			}),
		},
		"/transactions": map[string]any{
			"get": endpoint("List transactions", "Returns a list of transactions. Supports both cursor and offset pagination.", []param{
				queryParam("limit", "integer", "Number of transactions (cursor mode)", false),
				queryParam("before", "string", "Cursor for cursor-based pagination", false),
				queryParam("page", "integer", "Page number (offset mode)", false),
				queryParam("pageSize", "integer", "Page size (offset mode, max 100)", false),
			}, ref("CursorPaginatedTransactions")),
		},
		"/transactions/{hash}": map[string]any{
			"get": endpoint("Get transaction", "Returns transaction details by hash.", []param{
				pathParam("hash", "string", "Transaction hash"),
			}, ref("Transaction")),
		},
		"/transactions/{hash}/logs": map[string]any{
			"get": endpoint("Get transaction logs", "Returns event logs for a transaction.", []param{
				pathParam("hash", "string", "Transaction hash"),
			}, arrayOf(ref("Log"))),
		},
		"/transactions/{hash}/transfers": map[string]any{
			"get": endpoint("Get transaction transfers", "Returns token transfers in a transaction.", []param{
				pathParam("hash", "string", "Transaction hash"),
			}, arrayOf(ref("TokenTransfer"))),
		},
		"/transactions/{hash}/internal": map[string]any{
			"get": endpoint("Get internal transactions", "Returns internal transactions for a transaction.", []param{
				pathParam("hash", "string", "Transaction hash"),
			}, arrayOf(ref("InternalTransaction"))),
		},
		"/addresses/{address}": map[string]any{
			"get": endpoint("Get address info", "Returns address information including balance, tx count, and contract status.", []param{
				pathParam("address", "string", "Ethereum address"),
			}, ref("AddressInfo")),
		},
		"/addresses/{address}/transactions": map[string]any{
			"get": endpoint("Get address transactions", "Returns transactions for an address.", []param{
				pathParam("address", "string", "Ethereum address"),
				queryParam("limit", "integer", "Number of results", false),
				queryParam("before", "string", "Cursor", false),
			}, ref("CursorPaginatedTransactions")),
		},
		"/addresses/{address}/transfers": map[string]any{
			"get": endpoint("Get address transfers", "Returns token transfers for an address.", []param{
				pathParam("address", "string", "Ethereum address"),
				queryParam("page", "integer", "Page number", false),
				queryParam("pageSize", "integer", "Page size", false),
			}, ref("CursorPaginatedTokenTransfers")),
		},
		"/addresses/{address}/balances": map[string]any{
			"get": endpoint("Get address token balances", "Returns token balances for an address.", []param{
				pathParam("address", "string", "Ethereum address"),
			}, arrayOf(ref("Balance"))),
		},
		"/addresses/{address}/internal": map[string]any{
			"get": endpoint("Get address internal txs", "Returns internal transactions for an address.", []param{
				pathParam("address", "string", "Ethereum address"),
				queryParam("page", "integer", "Page number", false),
				queryParam("pageSize", "integer", "Page size", false),
			}, ref("OffsetPaginatedInternalTransactions")),
		},
		"/addresses/{address}/logs": map[string]any{
			"get": endpoint("Get address logs", "Returns event logs for an address.", []param{
				pathParam("address", "string", "Ethereum address"),
				queryParam("page", "integer", "Page number", false),
				queryParam("pageSize", "integer", "Page size", false),
			}, ref("OffsetPaginatedLogs")),
		},
		"/addresses/{address}/contract": map[string]any{
			"get": endpoint("Get contract details", "Returns contract details and ABI.", []param{
				pathParam("address", "string", "Contract address"),
			}, ref("Contract")),
		},
		"/tokens": map[string]any{
			"get": endpoint("List tokens", "Returns a paginated list of tokens.", []param{
				queryParam("page", "integer", "Page number", false),
				queryParam("pageSize", "integer", "Page size", false),
				queryParam("type", "string", "Token type filter (ERC20, ERC721)", false),
			}, ref("OffsetPaginatedTokens")),
		},
		"/tokens/{address}": map[string]any{
			"get": endpoint("Get token", "Returns token details.", []param{
				pathParam("address", "string", "Token contract address"),
			}, ref("Token")),
		},
		"/tokens/{address}/holders": map[string]any{
			"get": endpoint("Get token holders", "Returns holders of a token.", []param{
				pathParam("address", "string", "Token contract address"),
				queryParam("page", "integer", "Page number", false),
				queryParam("pageSize", "integer", "Page size", false),
			}, ref("OffsetPaginatedTokenHolders")),
		},
		"/tokens/{address}/transfers": map[string]any{
			"get": endpoint("Get token transfers", "Returns transfers for a token.", []param{
				pathParam("address", "string", "Token contract address"),
				queryParam("page", "integer", "Page number", false),
				queryParam("pageSize", "integer", "Page size", false),
			}, ref("OffsetPaginatedTokenTransfers")),
		},
		"/token-transfers": map[string]any{
			"get": endpoint("List all token transfers", "Returns all token transfers.", []param{
				queryParam("page", "integer", "Page number", false),
				queryParam("pageSize", "integer", "Page size", false),
			}, ref("OffsetPaginatedTokenTransfers")),
		},
		"/accounts": map[string]any{
			"get": endpoint("List top accounts", "Returns top accounts by transaction count.", []param{
				queryParam("page", "integer", "Page number", false),
				queryParam("pageSize", "integer", "Page size", false),
			}, ref("OffsetPaginatedAccounts")),
		},
		"/search": map[string]any{
			"get": endpoint("Search", "Search for blocks, transactions, or addresses.", []param{
				queryParam("q", "string", "Search query (block number, tx hash, or address)", true),
			}, map[string]any{
				"type": "object",
				"properties": map[string]any{
					"type": map[string]any{"type": "string", "enum": []string{"block", "transaction", "address"}},
					"data": map[string]any{"type": "object"},
				},
			}),
		},
		"/search/suggestions": map[string]any{
			"get": endpoint("Search suggestions", "Returns autocomplete suggestions.", []param{
				queryParam("q", "string", "Search query", true),
			}, ref("SearchSuggestionsResponse")),
		},
		"/stats": map[string]any{
			"get": endpoint("Get chain stats", "Returns chain statistics.", nil, ref("ChainStats")),
		},
		"/stats/tx-history": map[string]any{
			"get": endpoint("Get tx history", "Returns transaction history chart data.", []param{
				queryParam("interval", "integer", "Interval in seconds (default 60)", false),
				queryParam("limit", "integer", "Number of data points (default 30, max 100)", false),
			}, arrayOf(ref("TxHistoryPoint"))),
		},
		"/gas": map[string]any{
			"get": endpoint("Get gas prices", "Returns current gas price estimates.", nil, ref("GasPrices")),
		},
		"/chain-info": map[string]any{
			"get": endpoint("Get chain info", "Returns chain metadata from the node.", nil, ref("ChainInfo")),
		},
		"/sync": map[string]any{
			"get": endpoint("Get sync status", "Returns indexer sync status.", nil, map[string]any{
				"type": "object",
				"properties": map[string]any{
					"syncStatus":       ref("SyncStatus"),
					"latestChainBlock": map[string]any{"type": "integer"},
					"blocksRemaining":  map[string]any{"type": "integer"},
					"isSynced":         map[string]any{"type": "boolean"},
				},
			}),
		},
	}
}

func buildComponents() map[string]any {
	return map[string]any{
		"schemas": map[string]any{
			"Block": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"number":           map[string]any{"type": "integer"},
					"hash":             map[string]any{"type": "string"},
					"parentHash":       map[string]any{"type": "string"},
					"timestamp":        map[string]any{"type": "integer"},
					"gasUsed":          map[string]any{"type": "integer"},
					"gasLimit":         map[string]any{"type": "integer"},
					"baseFeePerGas":    map[string]any{"type": "integer"},
					"transactionCount": map[string]any{"type": "integer"},
					"size":             map[string]any{"type": "integer"},
					"miner":            map[string]any{"type": "string"},
					"extraData":        map[string]any{"type": "string"},
					"createdAt":        map[string]any{"type": "string", "format": "date-time"},
				},
			},
			"Transaction": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"hash":            map[string]any{"type": "string"},
					"blockNumber":     map[string]any{"type": "integer"},
					"blockTimestamp":   map[string]any{"type": "integer"},
					"txIndex":         map[string]any{"type": "integer"},
					"from":            map[string]any{"type": "string"},
					"to":              map[string]any{"type": "string"},
					"contractAddress": map[string]any{"type": "string"},
					"value":           map[string]any{"type": "string"},
					"gasUsed":         map[string]any{"type": "integer"},
					"gasPrice":        map[string]any{"type": "integer"},
					"txType":          map[string]any{"type": "integer"},
					"inputData":       map[string]any{"type": "string"},
					"status":          map[string]any{"type": "integer"},
					"txCategories":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					"createdAt":       map[string]any{"type": "string", "format": "date-time"},
				},
			},
			"Token": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"address":       map[string]any{"type": "string"},
					"symbol":        map[string]any{"type": "string"},
					"name":          map[string]any{"type": "string"},
					"decimals":      map[string]any{"type": "integer"},
					"tokenType":     map[string]any{"type": "string"},
					"totalSupply":   map[string]any{"type": "string"},
					"holderCount":   map[string]any{"type": "integer"},
					"transferCount": map[string]any{"type": "integer"},
				},
			},
			"TokenTransfer": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id":           map[string]any{"type": "integer"},
					"txHash":       map[string]any{"type": "string"},
					"logIndex":     map[string]any{"type": "integer"},
					"tokenAddress": map[string]any{"type": "string"},
					"from":         map[string]any{"type": "string"},
					"to":           map[string]any{"type": "string"},
					"value":        map[string]any{"type": "string"},
					"blockNumber":  map[string]any{"type": "integer"},
					"transferType": map[string]any{"type": "string"},
					"tokenType":    map[string]any{"type": "string"},
				},
			},
			"TokenHolder": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"address":    map[string]any{"type": "string"},
					"balance":    map[string]any{"type": "string"},
					"percentage": map[string]any{"type": "number"},
					"isContract": map[string]any{"type": "boolean"},
				},
			},
			"Balance": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"address":      map[string]any{"type": "string"},
					"tokenAddress": map[string]any{"type": "string"},
					"blockNumber":  map[string]any{"type": "integer"},
					"balance":      map[string]any{"type": "string"},
				},
			},
			"Log": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id":          map[string]any{"type": "integer"},
					"txHash":      map[string]any{"type": "string"},
					"logIndex":    map[string]any{"type": "integer"},
					"address":     map[string]any{"type": "string"},
					"topic0":      map[string]any{"type": "string"},
					"topic1":      map[string]any{"type": "string"},
					"topic2":      map[string]any{"type": "string"},
					"topic3":      map[string]any{"type": "string"},
					"data":        map[string]any{"type": "string"},
					"blockNumber": map[string]any{"type": "integer"},
				},
			},
			"InternalTransaction": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id":           map[string]any{"type": "integer"},
					"txHash":       map[string]any{"type": "string"},
					"blockNumber":  map[string]any{"type": "integer"},
					"traceAddress": map[string]any{"type": "string"},
					"from":         map[string]any{"type": "string"},
					"to":           map[string]any{"type": "string"},
					"value":        map[string]any{"type": "string"},
					"callType":     map[string]any{"type": "string"},
				},
			},
			"AddressInfo": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"address":    map[string]any{"type": "string"},
					"balance":    map[string]any{"type": "string"},
					"txCount":    map[string]any{"type": "integer"},
					"isContract": map[string]any{"type": "boolean"},
				},
			},
			"Contract": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"address":      map[string]any{"type": "string"},
					"bytecode":     map[string]any{"type": "string"},
					"creator":      map[string]any{"type": "string"},
					"creationTx":   map[string]any{"type": "string"},
					"blockNumber":  map[string]any{"type": "integer"},
					"isVerified":   map[string]any{"type": "boolean"},
					"contractName": map[string]any{"type": "string"},
					"abi":          map[string]any{"type": "array"},
					"sourceCode":   map[string]any{"type": "string"},
				},
			},
			"ChainStats": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"totalBlocks":       map[string]any{"type": "integer"},
					"totalTransactions": map[string]any{"type": "integer"},
					"totalAddresses":    map[string]any{"type": "integer"},
					"totalTokens":       map[string]any{"type": "integer"},
					"avgBlockTime":      map[string]any{"type": "number"},
				},
			},
			"SyncStatus": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"lastIndexedBlock": map[string]any{"type": "integer"},
					"isSyncing":        map[string]any{"type": "boolean"},
					"updatedAt":        map[string]any{"type": "string", "format": "date-time"},
				},
			},
			"ChainInfo": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"chainId":        map[string]any{"type": "string"},
					"chainIdDecimal": map[string]any{"type": "integer"},
					"networkId":      map[string]any{"type": "string"},
					"clientVersion":  map[string]any{"type": "string"},
					"latestBlock":    map[string]any{"type": "integer"},
					"gasPrice":       map[string]any{"type": "string"},
					"peerCount":      map[string]any{"type": "integer"},
					"isSyncing":      map[string]any{"type": "boolean"},
				},
			},
			"GasPrices": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"slow":      ref("GasPrice"),
					"normal":    ref("GasPrice"),
					"fast":      ref("GasPrice"),
					"updatedAt": map[string]any{"type": "string", "format": "date-time"},
				},
			},
			"GasPrice": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"price":       map[string]any{"type": "number"},
					"priceWei":    map[string]any{"type": "string"},
					"baseFee":     map[string]any{"type": "number"},
					"priorityFee": map[string]any{"type": "number"},
				},
			},
			"TxHistoryPoint": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"timestamp": map[string]any{"type": "integer"},
					"count":     map[string]any{"type": "integer"},
				},
			},
			"SearchSuggestion": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"type":  map[string]any{"type": "string"},
					"value": map[string]any{"type": "string"},
					"label": map[string]any{"type": "string"},
				},
			},
			"SearchSuggestionsResponse": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query":       map[string]any{"type": "string"},
					"suggestions": map[string]any{"type": "array", "items": ref("SearchSuggestion")},
				},
			},
			"AccountListItem": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"address":    map[string]any{"type": "string"},
					"balance":    map[string]any{"type": "string"},
					"txCount":    map[string]any{"type": "integer"},
					"isContract": map[string]any{"type": "boolean"},
				},
			},
			"ErrorResponse": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"error": map[string]any{"type": "string"},
				},
			},
			// Pagination wrappers
			"CursorPaginatedBlocks": cursorPaginatedSchema(ref("Block")),
			"CursorPaginatedTransactions": cursorPaginatedSchema(ref("Transaction")),
			"CursorPaginatedTokenTransfers": cursorPaginatedSchema(ref("TokenTransfer")),
			"OffsetPaginatedTransactions": offsetPaginatedSchema(ref("Transaction")),
			"OffsetPaginatedTokens": offsetPaginatedSchema(ref("Token")),
			"OffsetPaginatedTokenHolders": offsetPaginatedSchema(ref("TokenHolder")),
			"OffsetPaginatedTokenTransfers": offsetPaginatedSchema(ref("TokenTransfer")),
			"OffsetPaginatedInternalTransactions": offsetPaginatedSchema(ref("InternalTransaction")),
			"OffsetPaginatedLogs": offsetPaginatedSchema(ref("Log")),
			"OffsetPaginatedAccounts": offsetPaginatedSchema(ref("AccountListItem")),
		},
	}
}

// --- Spec helper types ---

type param = map[string]any

func ref(name string) map[string]any {
	return map[string]any{"$ref": "#/components/schemas/" + name}
}

func arrayOf(itemSchema map[string]any) map[string]any {
	return map[string]any{"type": "array", "items": itemSchema}
}

func pathParam(name, typ, desc string) param {
	return param{
		"name":        name,
		"in":          "path",
		"required":    true,
		"description": desc,
		"schema":      map[string]any{"type": typ},
	}
}

func queryParam(name, typ, desc string, required bool) param {
	return param{
		"name":        name,
		"in":          "query",
		"required":    required,
		"description": desc,
		"schema":      map[string]any{"type": typ},
	}
}

func endpoint(summary, description string, parameters []param, responseSchema map[string]any) map[string]any {
	op := map[string]any{
		"summary":     summary,
		"description": description,
		"responses": map[string]any{
			"200": map[string]any{
				"description": "Successful response",
				"content": map[string]any{
					"application/json": map[string]any{
						"schema": responseSchema,
					},
				},
			},
			"400": map[string]any{
				"description": "Bad request",
				"content": map[string]any{
					"application/json": map[string]any{
						"schema": ref("ErrorResponse"),
					},
				},
			},
			"404": map[string]any{
				"description": "Not found",
				"content": map[string]any{
					"application/json": map[string]any{
						"schema": ref("ErrorResponse"),
					},
				},
			},
			"429": map[string]any{
				"description": "Rate limit exceeded",
				"content": map[string]any{
					"application/json": map[string]any{
						"schema": ref("ErrorResponse"),
					},
				},
			},
			"500": map[string]any{
				"description": "Internal server error",
				"content": map[string]any{
					"application/json": map[string]any{
						"schema": ref("ErrorResponse"),
					},
				},
			},
		},
	}
	if parameters != nil {
		op["parameters"] = parameters
	}
	return op
}

func cursorPaginatedSchema(itemSchema map[string]any) map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"data":       map[string]any{"type": "array", "items": itemSchema},
			"nextCursor": map[string]any{"type": "string"},
			"hasMore":    map[string]any{"type": "boolean"},
		},
	}
}

func offsetPaginatedSchema(itemSchema map[string]any) map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"data":       map[string]any{"type": "array", "items": itemSchema},
			"total":      map[string]any{"type": "integer"},
			"page":       map[string]any{"type": "integer"},
			"pageSize":   map[string]any{"type": "integer"},
			"totalPages": map[string]any{"type": "integer"},
		},
	}
}
