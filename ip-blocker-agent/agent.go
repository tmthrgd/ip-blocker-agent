// Copyright 2016 Tom Thorogood. All rights reserved.
// Use of this source code is governed by a
// Modified BSD License license that can be found in
// the LICENSE file.

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

	var perms int
	flag.IntVar(&perms, "perms", 0600, "permissions")

	flag.Parse()

	if len(name) == 0 {
		fmt.Println("-name cannot be empty")
		os.Exit(1)
	}

	if err := blocker.Unlink(name); err != nil && err != blocker.ErrUnkownName {
		panic(err)
	}

	block, err := blocker.New(name, perms)
	if err != nil {
		panic(err)
	}

	defer block.Unlink()
	defer block.Close()

	fmt.Println(block)

	stdin := bufio.NewScanner(os.Stdin)

	for stdin.Scan() {
		line := stdin.Text()
		if len(line) == 0 {
			fmt.Printf("invalid input: %s\n", line)
			continue
		}

		switch line[0] {
		case '+':
			fallthrough
		case '-':
			if len(line) <= 1 {
				fmt.Printf("invalid input: %s\n", line)
				continue
			}

			if strings.Contains(line[1:], "/") {
				ip, ipnet, err := net.ParseCIDR(line[1:])
				if err != nil {
					fmt.Printf("invalid cidr mask: %s (%v)\n", line[1:], err)
					continue
				}

				switch line[0] {
				case '+':
					err = block.InsertRange(ip, ipnet)
				case '-':
					err = block.RemoveRange(ip, ipnet)
				}
			} else {
				ip := net.ParseIP(line[1:])
				if ip == nil {
					fmt.Printf("invalid ip address: %s\n", line[1:])
					continue
				}

				switch line[0] {
				case '+':
					err = block.Insert(ip)
				case '-':
					err = block.Remove(ip)
				}
			}

			if err != nil {
				panic(err)
			}

			if !block.IsBatching() {
				fmt.Println(block)
			}
		case '!':
			if len(line) != 1 {
				fmt.Printf("invalid input: %s\n", line)
				continue
			}

			if err = block.Clear(); err != nil {
				panic(err)
			}

			fmt.Println(block)
		case 'b':
			if len(line) != 1 && !strings.EqualFold(line, "batch") {
				fmt.Printf("invalid input: %s\n", line)
				continue
			}

			if err = block.Batch(); err != nil {
				if err == blocker.ErrAlreadyBatching {
					fmt.Println(err)
				} else {
					panic(err)
				}
			}
		case 'B':
			if len(line) != 1 && !strings.EqualFold(line, "batch") {
				fmt.Printf("invalid input: %s\n", line)
				continue
			}

			if err = block.Commit(); err != nil {
				if err == blocker.ErrNotBatching {
					fmt.Println(err)
					continue
				} else {
					panic(err)
				}
			}

			fmt.Println(block)
		case 'q':
			fallthrough
		case 'Q':
			if len(line) == 1 || strings.EqualFold(line, "quit") {
				return
			}

			fmt.Printf("invalid input: %s\n", line)
		default:
			fmt.Printf("invalid operation: %c\n", line[0])
		}
	}

	if err = stdin.Err(); err != nil {
		panic(err)
	}
}
