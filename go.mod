module gitlab.com/niharokz/arthik

go 1.23

require (
	golang.org/x/crypto v0.31.0
	gopkg.in/yaml.v3 v3.0.1
)

// Fetched through their GitHub mirrors (same approach as Omnimo/Kronos), so the
// build works where gopkg.in / golang.org are blocked.
replace gopkg.in/yaml.v3 => github.com/go-yaml/yaml v0.0.0-20220527083530-f6f7691b1fde

replace golang.org/x/crypto => github.com/golang/crypto v0.31.0
