# ip-blocker-agent

[![GoDoc](https://godoc.org/github.com/tmthrgd/ip-blocker-agent?status.svg)](https://godoc.org/github.com/tmthrgd/ip-blocker-agent)
[![Build Status](https://travis-ci.org/tmthrgd/ip-blocker-agent.svg?branch=master)](https://travis-ci.org/tmthrgd/ip-blocker-agent)

ip-blocker-agent is the other half of
[tmthrgd/nginx-ip-blocker](https://github.com/tmthrgd/nginx-ip-blocker).

## Download

```
go get -u github.com/tmthrgd/ip-blocker-agent/...
```

## Run

ip-blocker-agent accepts two flags:

-name which defaults to '/ngx-ip-blocker' and specifies the name of the shared memory.

-perms which defaults to 0600 and allows the shared memory permissions to be specified.

ip-blocker-agent has one subcommand:

- unlink which removes a previously created blocklist at the specified name.

## User interface (on stdin)

+192.0.2.0 add single IPv4 address.  
+192.0.2.0/24 add IPv4 address range.  
+2001:db8:: add single IPv6 address.  
+2001:db8::/96 add IPv6 address range.  
+2001:db8::/32 add IPv6 route range.*  
-ip[/block] does the inverse of the above operations and removes the IP address(es).  
! clears all IP addresses.  
b starts batching and will withhold all updates until batching is ended.  
B ends batching.  
q quits the program.

*: Due to the large size of the IPv6 address space, a second IPv6 table is included that holds
all IPv6 blocks /64 or larger. /64 corresponds to the routing mask of an IPv6 address and excludes
the interface identifier.

General unicast address format (routing prefix size varies)

| bits      | 48 (or more)   | 16 (or fewer) | 64                   |
|:---------:|:--------------:|:-------------:|:--------------------:|
| **field** | routing prefix | subnet id     | interface identifier |

## Tips and Tricks

Block all Tor Exit Nodes:

```
cat <(echo b && curl https://check.torproject.org/exit-addresses | grep ExitAddress | cut -d ' ' -f2 | awk '$0="+"$0' && echo B) /dev/stdin | ip-blocker-agent
```

## License

Unless otherwise noted, the ip-blocker-agent source files are distributed under the Modified BSD License
found in the LICENSE file.
