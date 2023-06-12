package app

func ToSystemArch(arch ArchType) string {
	switch arch {
	case ArchType_AMD64:
		return "amd64"
	case ArchType_ARM64:
		return "arm64"
	}
	return ""
}
