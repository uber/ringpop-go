struct ping {
    1: required string key,
}

struct pong {
    1: required string source,
}

service PingPong {
    pong Ping(1: ping request)
}
