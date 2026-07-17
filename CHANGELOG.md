# [1.2.0](https://github.com/tdesaules/coredns-service-discovery/compare/v1.1.0...v1.2.0) (2026-07-17)


### Bug Fixes

* harden store validation and return defensive copies ([e344022](https://github.com/tdesaules/coredns-service-discovery/commit/e3440223f2f5327ff3c1f78e6e87175dd411e330))
* reject A queries with more than 3 relative labels ([a6fb788](https://github.com/tdesaules/coredns-service-discovery/commit/a6fb788376995ffc173a22a8623e753b91f1e0e6))


### Features

* warn when no sources are configured ([8e10323](https://github.com/tdesaules/coredns-service-discovery/commit/8e103239577152989896b24ea2c224ccf5eea007))

# [1.1.0](https://github.com/tdesaules/coredns-service-discovery/compare/v1.0.0...v1.1.0) (2026-07-16)


### Features

* add real fallthrough support, SRV protocol filter, and store validation ([f5cc2b6](https://github.com/tdesaules/coredns-service-discovery/commit/f5cc2b60a88969a99c261929e2184a5f4976ff20))

# 1.0.0 (2026-07-16)


### Bug Fixes

* install golangci-lint v2 via go install instead of action ([80d6aa9](https://github.com/tdesaules/coredns-service-discovery/commit/80d6aa9c45b02fa83225ab1a20aa948540c9bdef))
* migrate golangci-lint to v2, fix gosec errcheck and revive issues ([0c1a7ae](https://github.com/tdesaules/coredns-service-discovery/commit/0c1a7ae826d42658f0657c2880f109563070ae43))


### Features

* initial release ([de380e2](https://github.com/tdesaules/coredns-service-discovery/commit/de380e2006f52fd42d8af05713e72a41bf9df256))
