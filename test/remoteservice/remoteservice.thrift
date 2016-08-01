include "./shared.thrift"

service RemoteService {
  void RemoteCall(1: shared.UUID id)
}
