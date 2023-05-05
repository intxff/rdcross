package policy

type None struct{}

func NewPolicyNone() None {
    return None{}
}

func (p None) Select(g map[string][]string) string {
    return ""
}

func (p None) Type() PolicyType {
    return TypeNone
}
