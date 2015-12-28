struct S {
  1: binary s1
}

service S1 {
  binary M1(1: binary bits)
}

service S2 extends S1 {
  S M2(1: S s)
}

service S3 extends S2 {
  void M3()
}

//Go code: service_extend/test.go
// package service_extend
// var _ = TChanS3(nil).M1
// var _ = TChanS3(nil).M2
// var _ = TChanS3(nil).M3
