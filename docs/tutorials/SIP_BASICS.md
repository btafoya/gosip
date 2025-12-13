# SIP Server Basics Tutorial

> **Note**: This is educational reference material demonstrating SIP protocol concepts.
> For production implementation, see the `pkg/sip/` package which uses the sipgo library.

## Overview

This tutorial demonstrates basic SIP server concepts in Go. The code examples are simplified for learning purposes and should not be used in production.

---

## Step 1: Creating a Basic SIP Server

A minimal SIP server listens on UDP port 5060 and responds to incoming requests.

```go
package main

import (
    "fmt"
    "net"
)

func main() {
    addr := net.UDPAddr{
        Port: 5060,
        IP:   net.ParseIP("0.0.0.0"),
    }
    conn, err := net.ListenUDP("udp", &addr)
    if err != nil {
        fmt.Println(err)
        return
    }
    defer conn.Close()

    fmt.Println("SIP Server started on", addr)

    buf := make([]byte, 1024)
    for {
        n, addr, err := conn.ReadFromUDP(buf)
        if err != nil {
            fmt.Println("Error:", err)
            continue
        }

        // Parse SIP message
        message := string(buf[:n])
        fmt.Println("Received message from", addr, ":", message)

        // Send response
        response := "SIP/2.0 200 OK\r\n\r\n"
        conn.WriteToUDP([]byte(response), addr)
    }
}
```

This creates a simple SIP server that:
- Listens on UDP port 5060
- Reads incoming SIP requests
- Responds with 200 OK to all requests

---

## Step 2: Adding Digest Authentication

SIP uses Digest authentication (RFC 2617) to validate clients. This involves:
1. Client sends request without credentials
2. Server responds with 401 Unauthorized + challenge (realm, nonce)
3. Client sends request with Authorization header containing MD5 hash
4. Server validates the hash

```go
package main

import (
    "bufio"
    "crypto/md5"
    "errors"
    "fmt"
    "net"
    "regexp"
    "strings"
)

// Device credentials (in production, use database)
var devices = map[string]string{
    "device1": "password1",
    "device2": "password2",
}

// Regular expression for extracting Digest authentication credentials
var digestRegexp = regexp.MustCompile(
    `username="([^"]+)",\s*realm="([^"]+)",\s*nonce="([^"]+)",\s*uri="([^"]+)",\s*response="([^"]+)"`,
)

func main() {
    addr := net.UDPAddr{
        Port: 5060,
        IP:   net.ParseIP("0.0.0.0"),
    }
    conn, err := net.ListenUDP("udp", &addr)
    if err != nil {
        fmt.Println(err)
        return
    }
    defer conn.Close()

    fmt.Println("SIP Server started on", addr)

    buf := make([]byte, 1024)
    for {
        n, remoteAddr, err := conn.ReadFromUDP(buf)
        if err != nil {
            fmt.Println("Error:", err)
            continue
        }

        message := string(buf[:n])
        fmt.Println("Received message from", remoteAddr, ":", message)

        // Check for authentication credentials
        if strings.Contains(message, "Authorization") {
            username, _, err := extractCredentials(message)
            if err != nil {
                // Send 401 Unauthorized response
                response := "SIP/2.0 401 Unauthorized\r\n"
                response += "WWW-Authenticate: Digest realm=\"MyRealm\", nonce=\"MyNonce\"\r\n\r\n"
                conn.WriteToUDP([]byte(response), remoteAddr)
                continue
            }
            fmt.Println("Authenticated user:", username)
        }

        // Send 200 OK response
        response := "SIP/2.0 200 OK\r\n\r\n"
        conn.WriteToUDP([]byte(response), remoteAddr)
    }
}

// extractCredentials extracts and validates Digest authentication credentials
func extractCredentials(message string) (string, string, error) {
    scanner := bufio.NewScanner(strings.NewReader(message))
    for scanner.Scan() {
        line := scanner.Text()
        if strings.HasPrefix(line, "Authorization: Digest") {
            match := digestRegexp.FindStringSubmatch(line)
            if len(match) > 0 {
                username := match[1]
                password := devices[username]
                if password == "" {
                    return "", "", errors.New("device not registered")
                }

                realm := match[2]
                nonce := match[3]
                uri := match[4]
                response := match[5]

                // Calculate expected response (RFC 2617)
                ha1 := fmt.Sprintf("%x", md5.Sum([]byte(
                    fmt.Sprintf("%s:%s:%s", username, realm, password),
                )))
                ha2 := fmt.Sprintf("%x", md5.Sum([]byte(
                    fmt.Sprintf("%s:%s", "REGISTER", uri),
                )))
                expectedResponse := fmt.Sprintf("%x", md5.Sum([]byte(
                    fmt.Sprintf("%s:%s:%s", ha1, nonce, ha2),
                )))

                if response != expectedResponse {
                    return "", "", errors.New("invalid credentials")
                }
                return username, password, nil
            }
        }
    }
    return "", "", errors.New("no credentials found")
}
```

---

## Key Concepts

### SIP Methods
- **REGISTER** - Device registration with server
- **INVITE** - Initiate a call
- **ACK** - Acknowledge call setup
- **BYE** - End a call
- **CANCEL** - Cancel pending request
- **OPTIONS** - Query capabilities

### SIP Response Codes
- **1xx** - Provisional (100 Trying, 180 Ringing)
- **2xx** - Success (200 OK)
- **3xx** - Redirection
- **4xx** - Client Error (401 Unauthorized, 404 Not Found)
- **5xx** - Server Error
- **6xx** - Global Failure

### Digest Authentication Flow
```
Client                          Server
   |                               |
   |-------- REGISTER ------------>|
   |                               |
   |<------ 401 Unauthorized ------|
   |        WWW-Authenticate:      |
   |        Digest realm, nonce    |
   |                               |
   |-------- REGISTER ------------>|
   |        Authorization:         |
   |        Digest username,       |
   |        response (MD5 hash)    |
   |                               |
   |<-------- 200 OK --------------|
```

---

## Production Implementation

For production use, GoSIP uses the [sipgo](https://github.com/emiago/sipgo) library which provides:
- Full SIP RFC compliance
- UDP, TCP, TLS, WebSocket transports
- Dialog management
- Transaction handling
- NAT traversal

See `pkg/sip/` for the production implementation.

---

## References

- [RFC 3261](https://tools.ietf.org/html/rfc3261) - SIP: Session Initiation Protocol
- [RFC 2617](https://tools.ietf.org/html/rfc2617) - HTTP Authentication: Basic and Digest Access Authentication
- [sipgo Documentation](https://github.com/emiago/sipgo)
