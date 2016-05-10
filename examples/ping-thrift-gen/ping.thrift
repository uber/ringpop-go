struct ping {
    1: required string key,
}

struct pong {
    1: required string message,
    2: required string from_,
    3: optional string pheader,
}

service PingPongService {
    pong Ping(1: ping request)
}
