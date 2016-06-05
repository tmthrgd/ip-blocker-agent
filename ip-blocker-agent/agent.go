// Copyright 2016 Tom Thorogood. All rights reserved.
// Use of this source code is governed by a
// Modified BSD License license that can be found in
// the LICENSE file.

// An efficient shared memory IP blocking system for nginx.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	blocker "github.com/tmthrgd/ip-blocker-agent"
)

type octalValue int

func (o *octalValue) Set(s string) error {
	v, err := strconv.ParseInt(s, 8, 64)
	*o = octalValue(v)
	return err
}

func (o *octalValue) Get() interface{} {
	return int(*o)
}

func (o *octalValue) String() string {
	return fmt.Sprintf("%#o", *o)
}

func printServer(server *blocker.Server) {
	ip4, ip6, ip6r, err := server.Count()
	if err != nil {
		panic(err)
	}

	fmt.Printf("IP4: %d, IP6: %d, IP6 routes: %d\n", ip4, ip6, ip6r)
}

func main() {
	var name string
	flag.StringVar(&name, "name", "/ngx-ip-blocker", "the shared memory name")

	perms := 0600
	flag.Var((*octalValue)(&perms), "perms", "permissions")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s [-name <path>] [-perms <perms>] [unlink]\n", os.Args[0])
		flag.PrintDefaults()
	}

	flag.Parse()

	if len(name) == 0 {
		fmt.Println("-name cannot be empty")
		os.Exit(1)
	}

	switch flag.NArg() {
	case 0:
	case 1:
		if flag.Arg(0) != "unlink" {
			flag.Usage()
			os.Exit(1)
		}

		if err := blocker.Unlink(name); err != nil {
			if os.IsNotExist(err) {
				fmt.Println(err)
				os.Exit(1)
			} else {
				panic(err)
			}
		}

		return
	default:
		flag.Usage()
		os.Exit(1)
	}

	server, err := blocker.New(name, os.FileMode(perms))
	if err != nil {
		if os.IsExist(err) {
			fmt.Println(err)
			os.Exit(1)
		} else {
			panic(err)
		}
	}

	defer server.Unlink()
	defer server.Close()

	printServer(server)

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
					err = server.InsertRange(ip, ipnet)
				case '-':
					err = server.RemoveRange(ip, ipnet)
				}
			} else {
				ip := net.ParseIP(line[1:])
				if ip == nil {
					fmt.Printf("invalid ip address: %s\n", line[1:])
					continue
				}

				switch line[0] {
				case '+':
					err = server.Insert(ip)
				case '-':
					err = server.Remove(ip)
				}
			}

			if err != nil {
				panic(err)
			}

			if !server.IsBatching() {
				printServer(server)
			}
		case '!':
			if len(line) != 1 {
				fmt.Printf("invalid input: %s\n", line)
				continue
			}

			if err = server.Clear(); err != nil {
				panic(err)
			}

			if !server.IsBatching() {
				printServer(server)
			}
		case 'b':
			if len(line) != 1 && !strings.EqualFold(line, "batch") {
				fmt.Printf("invalid input: %s\n", line)
				continue
			}

			if err = server.Batch(); err != nil {
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

			if err = server.Commit(); err != nil {
				if err == blocker.ErrNotBatching {
					fmt.Println(err)
					continue
				} else {
					panic(err)
				}
			}

			printServer(server)
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
