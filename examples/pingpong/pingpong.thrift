struct ping {
    1: string key,
}

struct pong {
    1: string source,
}

service PingPong {
    pong Ping(1: ping request)
}
