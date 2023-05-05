package set

type set interface {
	Value() []string
	Members() [][]string
}

// check if these element can be a set
func IsSet(s ...set) (string, bool) {
	t := make(map[string]struct{})
	for _, i := range s {
		for _, j := range i.Value() {
			if _, exist := t[j]; exist {
				return j, false
			}
			t[j] = struct{}{}
		}
	}
	return "", true
}

// check if members of B belong to A
func IsSubSet(s set, m set) (string, bool) {
	t := make(map[string]struct{})
	for _, v := range s.Value() {
		t[v] = struct{}{}
	}

	for _, i := range m.Members() {
		for _, j := range i {
			if _, exist := t[j]; !exist {
				return j, false
			}
		}
	}
	return "", true
}

// check if dst is a member of s...
func IsInSet(dst string, s ...set) bool {
	t := make(map[string]struct{})
	for _, i := range s {
		for _, j := range i.Value() {
			t[j] = struct{}{}
		}
	}

	if _, exist := t[dst]; !exist {
		return false
	}
	return true
}
