package policy

type Policy interface {
    Type() PolicyType
    Select(map[string][]string) string
}

type PolicyType string

const (
    TypeNone PolicyType = "none"
)
