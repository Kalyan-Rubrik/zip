package zip

import (
	"bufio"
	"fmt"
	"io"
)

var done = fmt.Errorf("no more ZIP file entries")

func HasNoMoreEntries(err error) bool {
	return err == done
}

type Iterator struct {
	r         io.ReaderAt   // io handle for reading the underlying ZIP file
	Comment   string        // ZIP file comment, if any
	buf       *bufio.Reader // handle for section reader buffer
	size      int64
	dirOffset uint64
}

func NewIterator(r io.ReaderAt, size int64) (*Iterator, error) {
	if size < 0 {
		return nil, fmt.Errorf("zip: size cannot be negative")
	}
	iter := new(Iterator)
	if err := iter.init(r, size); err != nil {
		return nil, err
	}
	return iter, nil
}

func (it *Iterator) init(r io.ReaderAt, size int64) error {
	end, err := readDirectoryEnd(r, size)
	if err != nil {
		return err
	}
	it.dirOffset = end.directoryOffset
	it.r = r
	it.size = size
	it.Comment = end.comment
	return it.Reset()
}

func (it *Iterator) Reset() error {
	rs := io.NewSectionReader(it.r, 0, it.size)
	if _, err := rs.Seek(int64(it.dirOffset), io.SeekStart); err != nil {
		return err
	}
	it.buf = bufio.NewReader(rs)
	return nil
}

func (it *Iterator) Next() (*File, error) {
	f := &File{zip: new(Reader), zipr: it.r}
	err := readDirectoryHeader(f, it.buf)
	if err == ErrFormat || err == io.ErrUnexpectedEOF {
		return nil, done
	}
	if err != nil {
		return nil, err
	}
	return f, nil
}
