package common

import "os"

func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil || os.IsExist(err)
}

func Mkdir(path string, existOk bool) error {
	if existOk {
		if err := os.MkdirAll(path, os.ModePerm); err != nil {
			return err
		}
	} else {
		if err := os.Mkdir(path, os.ModePerm); err != nil {
			return err
		}
	}
	return nil
}

func LoadFile(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer func(file *os.File) {
		_ = file.Close()
	}(file)
	info, _ := file.Stat()
	buf := make([]byte, info.Size())
	_, _ = file.Read(buf)
	return buf, nil

}
