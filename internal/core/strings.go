package core

func ContainsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr || len(substr) == 0 ||
			(func() bool {
				sl := []rune(s)
				subl := []rune(substr)
				for i := 0; i <= len(sl)-len(subl); i++ {
					match := true
					for j := range subl {
						cs := sl[i+j]
						csub := subl[j]
						if cs >= 'A' && cs <= 'Z' {
							cs += 32
						}
						if csub >= 'A' && csub <= 'Z' {
							csub += 32
						}
						if cs != csub {
							match = false
							break
						}
					}
					if match {
						return true
					}
				}
				return false
			})())
}
