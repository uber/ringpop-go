Test no args returns exit code 1:

  $  "$BASE_DIR"/lint-warn
  ERROR: Expect lint exclude file as argument
  [1]


Test lint with only warnings returns 0 exit code:

  $  cat "$BASE_DIR"/test/test-lint-ok.log |"$BASE_DIR"/lint-warn "$BASE_DIR"/test/test-excludes
  WARNING: ./swim/member.go:53: address passes Lock by value: swim.Member
  WARNING: ./swim/member.go:57: incarnation passes Lock by value: swim.Member
  WARNING: ./swim/test_utils.go:135: range var expected copies Lock: swim.Member


Test lint with a mix of warnings/errors returns exit code 1:

  $  cat "$BASE_DIR"/test/test-lint-mix.log |"$BASE_DIR"/lint-warn "$BASE_DIR"/test/test-excludes
  ERROR: ./swim/disseminator.go:107: range var member copies Lock: swim.Member
  WARNING: ./swim/member.go:53: address passes Lock by value: swim.Member
  WARNING: ./swim/member.go:57: incarnation passes Lock by value: swim.Member
  ERROR: ./swim/memberlist.go:128: Pingable passes Lock by value: swim.Member
  ERROR: ./swim/stats_test.go:101: range var member copies Lock: swim.Member
  WARNING: ./swim/test_utils.go:135: range var expected copies Lock: swim.Member
  [1]


Test lint with only errors returns exit code 1:

  $  cat "$BASE_DIR"/test/test-lint-all-fail.log |"$BASE_DIR"/lint-warn "$BASE_DIR"/test/test-excludes
  ERROR: ./swim/disseminator.go:107: range var member copies Lock: swim.Member
  ERROR: ./swim/memberlist.go:128: Pingable passes Lock by value: swim.Member
  ERROR: ./swim/stats_test.go:101: range var member copies Lock: swim.Member
  [1]
