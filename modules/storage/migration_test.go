package storage

import (
	"context"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigration(t *testing.T) {
	srcDir, err := ioutil.TempDir("", "gitea-storage-migration-src")
	require.NoError(t, err)
	defer os.RemoveAll(srcDir)

	dstDir, err := ioutil.TempDir("", "gitea-storage-migration-dst")
	require.NoError(t, err)
	defer os.RemoveAll(dstDir)

	for _, fn := range []string{"1", "2"} {
		assert.NoError(t, ioutil.WriteFile(filepath.Join(srcDir, fn), []byte("content"), 0666))
	}

	ctx := context.Background()
	src, err := OpenBucket(ctx, srcDir)
	require.NoError(t, err)
	defer src.Close()
	dst, err := OpenBucket(ctx, dstDir)
	require.NoError(t, err)
	defer dst.Close()

	migration := NewMigration(src, dst)
	assert.NoError(t, migration.Run(ctx))

	iter := dst.List(nil)
	count := 0
	for {
		_, err := iter.Next(ctx)
		if err == io.EOF {
			break
		}
		if assert.NoError(t, err) {
			count++
		}
	}
	assert.Equal(t, 2, count)
}

func TestMigrationPrefix(t *testing.T) {
	srcDir, err := ioutil.TempDir("", "gitea-storage-migration-src")
	require.NoError(t, err)
	defer os.RemoveAll(srcDir)

	dstDir, err := ioutil.TempDir("", "gitea-storage-migration-dst")
	require.NoError(t, err)
	defer os.RemoveAll(dstDir)

	for _, subDir := range []string{"1", "2"} {
		assert.NoError(t, os.Mkdir(filepath.Join(srcDir, subDir), 0755))
		assert.NoError(t, ioutil.WriteFile(filepath.Join(srcDir, subDir, "1"), []byte("content"), 0666))
	}

	ctx := context.Background()
	src, err := OpenBucket(ctx, srcDir)
	require.NoError(t, err)
	defer src.Close()
	dst, err := OpenBucket(ctx, dstDir)
	require.NoError(t, err)
	defer dst.Close()

	migration := NewMigration(src, dst)
	migration.SetPrefix("1")
	assert.NoError(t, migration.Run(ctx))

	iter := dst.List(nil)
	count := 0
	for {
		obj, err := iter.Next(ctx)
		if err == io.EOF {
			break
		}
		if assert.NoError(t, err) {
			count++
			assert.True(t, strings.HasPrefix(obj.Key, "1"))
		}
	}
	assert.Equal(t, 1, count)
}
