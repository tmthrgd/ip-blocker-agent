// Copyright 2016 Tom Thorogood. All rights reserved.
// Use of this source code is governed by a
// Modified BSD License license that can be found in
// the LICENSE file.

// An efficient shared memory IP blocking client.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"

	blocker "github.com/tmthrgd/ip-blocker-agent"
)

func main() {
	var name string
	flag.StringVar(&name, "name", "/ngx-ip-blocker", "the shared memory name")

	flag.Parse()

	if len(name) == 0 {
		fmt.Println("-name cannot be empty")
		os.Exit(1)
	}

	block, err := blocker.Open(name)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println(err)
			os.Exit(1)
		} else {
			panic(err)
		}
	}

	defer block.Close()

	stdin := bufio.NewScanner(os.Stdin)

	for stdin.Scan() {
		line := stdin.Text()
		if len(line) == 0 {
			fmt.Printf("invalid input: %s\n", line)
			continue
		}

		switch line[0] {
		case 'q':
			fallthrough
		case 'Q':
			if len(line) == 1 || strings.EqualFold(line, "quit") {
				return
			}

			fmt.Printf("invalid input: %s\n", line)
		default:
			ip := net.ParseIP(line)
			if ip == nil {
				fmt.Printf("invalid ip address: %s\n", line)
				continue
			}

			has, err := block.Contains(ip)
			if err != nil {
				panic(err)
			}

			fmt.Printf("%t\n", has)
		}
	}

	if err = stdin.Err(); err != nil {
		panic(err)
	}
}
