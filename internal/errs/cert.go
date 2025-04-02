package errs

import "errors"

var (
	NoCert         = errors.New("cert: no certificate")
	DeprecatedCert = errors.New("cert: deprecated")
)
