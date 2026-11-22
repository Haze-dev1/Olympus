//go:build !linux

package activate

func readConfig(string, int, string) ([]byte, error) {
	return nil, errReadConfig
}
