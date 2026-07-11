# Test audit summary

Static inventory contains 40 `_test.go` files, 158 direct `Test*` functions, and 16 Ginkgo `It` cases. This generator does not mark any current pass; execution status belongs only in machine-readable test results produced by the final test matrix.

Missing measured-path coverage at takeover: real PostgreSQL integration; race/concurrent duplicate reserve/settle; duplicate distinct-ID settlement; expiry/late settlement; gateway→provider→usage→settlement; fault campaign; multi-replica settlement; Kind Helm upgrade/rollback/uninstall; streaming/timeouts/429s; native Envoy admission.
