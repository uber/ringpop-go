include "./shared.thrift"
include "./unused.thrift"

service RemoteService {
  void RemoteCall(1: shared.UUID id)
}
