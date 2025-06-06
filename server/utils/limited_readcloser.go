// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package utils

import (
	"io"

	"github.com/mattermost/mattermost-plugin-jira/server/utils/types"
)

type LimitedReadCloser struct {
	TotalRead types.ByteSize

	reader   io.Reader
	closer   io.Closer
	preClose func(*LimitedReadCloser) error
}

func NewLimitedReadCloser(rc io.ReadCloser, limit types.ByteSize, preClose func(*LimitedReadCloser) error) io.ReadCloser {
	lrc := &LimitedReadCloser{
		reader:   rc,
		closer:   rc,
		preClose: preClose,
	}
	if limit >= 0 {
		lrc.reader = io.LimitReader(rc, int64(limit))
	}
	return lrc
}

func (lrc *LimitedReadCloser) Read(data []byte) (int, error) {
	n, err := lrc.reader.Read(data)
	lrc.TotalRead += types.ByteSize(n)
	return n, err
}

func (lrc *LimitedReadCloser) Close() error {
	if lrc.preClose != nil {
		err := lrc.preClose(lrc)
		if err != nil {
			return err
		}
	}
	return lrc.closer.Close()
}
