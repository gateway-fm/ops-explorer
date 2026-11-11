package main

import (
	"context"
	"explorer/internal/authv2"
	"explorer/internal/authv2admin"
	"flag"
	"fmt"
	"io"
	"os"
)

func main() {
	file := flag.String("source", "", "approved source file")
	digest := flag.String("sha256", "", "approved exact file SHA-256")
	action := flag.String("action", "", "migrate, initialize or drained-recover")
	drained := flag.Bool("traffic-drained", false, "acknowledge all serving instances are drained for recovery")
	flag.Parse()
	sha, e := authv2.ExpectedSHA(*digest)
	if e == nil {
		var raw []byte
		var input *os.File
		input, e = os.Open(*file)
		if e == nil {
			raw, e = io.ReadAll(io.LimitReader(input, (64<<10)+1))
			_ = input.Close()
		}
		if e == nil && len(raw) <= 64<<10 {
			var source *authv2.Source
			source, e = authv2.ParseSource(raw, sha)
			if e == nil {
				e = authv2admin.Run(context.Background(), os.Getenv("EXPLORER_SSO_OWNER_DSN"), source, *action, *drained)
			}
		} else {
			e = authv2admin.ErrInvalid
		}
	}
	if e != nil {
		fmt.Fprintln(os.Stderr, "offline SSO administration failed")
		os.Exit(1)
	}
	fmt.Println("offline SSO administration complete")
}
