Steps for code re-generation:
-----------------------------

a). remoteservice_test imports generated go-code for thrift files under `.gen/go/...`. To re-generate apache thrift and ringpop code run:

```
    thrift-gen --generateThrift --outputDir .gen/go --inputFile remoteservice.thrift 
        --template github.com/uber/ringpop-go/ringpop.thrift-gen 
        -packagePrefix github.com/uber/ringpop-go/test/remoteservice/.gen/go/
```

b). remoteservice_test also consumes mocks for TChanRemoteService. To re-generate mocks run:

```
    mockery -dir=.gen/go/remoteservice -name=TChanRemoteService
```