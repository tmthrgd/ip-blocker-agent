# ip-blocker-client

[![GoDoc](https://godoc.org/github.com/tmthrgd/ip-blocker-agent?status.svg)](https://godoc.org/github.com/tmthrgd/ip-blocker-agent)
[![Build Status](https://travis-ci.org/tmthrgd/ip-blocker-agent.svg?branch=master)](https://travis-ci.org/tmthrgd/ip-blocker-agent)

## Download

```
go get -u github.com/tmthrgd/ip-blocker-agent/...
```

## Run

ip-blocker-client accepts one flag:

-name which defaults to '/ngx-ip-blocker' and specifies the name of the shared memory.

ip-blocker-client can be run with a single ip address specified like so:

```
ip-blocker-client 192.0.2.0
```

In this case, ip-blocker-client will exit with a status of 0 if the IP address is in the blocklist
and a status of 1 if it is not.

## User interface (on stdin)

192.0.2.0 queries a single IPv4 address.  
2001:db8:: queries a single IPv6 address.  
? prints information about the shared memory mapping.  
q quits the program.

## License

Unless otherwise noted, the ip-blocker-agent source files are distributed under the Modified BSD License
found in the LICENSE file.
