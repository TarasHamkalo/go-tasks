package logging

import "os"

// FileAppender implements Appender, should be closed after use.
type FileAppender struct {
	FilePath string

	file *os.File
}

// NewFileAppender Returns FileAppender and error if any occurred during
// file opening. File exists, then is opened in append mode, otherwise created.
func NewFileAppender(path string) (*FileAppender, error) {
	file, err := os.OpenFile(
		path,
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0644,
	)

	if err != nil {
		return nil, err
	}

	return &FileAppender{FilePath: path, file: file}, nil
}

func (f *FileAppender) Append(data []byte) error {
	_, err := f.file.Write(data)
	if err != nil {
		return err
	}

	_, err = f.file.Write([]byte{'\n'})
	return err
}

func (f *FileAppender) Close() error {
	return f.file.Close()
}
