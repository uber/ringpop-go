struct ping {
    1: required string key,
}

struct pong {
    1: required string source,
}

exception PingError {}

service PingPong {
    pong Ping(1: ping request) throws (1: PingError pingError)
}
