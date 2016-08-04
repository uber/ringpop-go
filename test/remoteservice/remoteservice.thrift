include "./shared.thrift"

// the unused file contains types that are not used in service endpoints. These
// types might actually be used in embedded structs here, but since the
// generated code could potentially contain the import of an unused package in
// the RingpopAdapter. To prevent this the generator uses `GoUnusedProtection__`
include "./unused.thrift"

service RemoteService {
  void RemoteCall(1: shared.Name name)
}
