// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/urfave/cli"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
)

// CmdStorageMigrate represents the available storage sub-command.
var (
	CmdStorageMigrate = cli.Command{
		Name:        "storage-migrate",
		Usage:       "Migrate the storage",
		Description: "This is a command for migrating the storage, from local to cloud and vice versa.",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "bucket",
				Usage: "Cloud provider bucket url",
			},
			cli.BoolFlag{
				Name:  "local",
				Usage: "Migrate to local storage",
			},
		},
		Action: runStorageMigrate,
	}
)

func runStorageMigrate(c *cli.Context) error {
	setting.NewContext()
	log.Trace("AppPath: %s", setting.AppPath)
	log.Trace("AppWorkPath: %s", setting.AppWorkPath)
	log.Trace("Custom path: %s", setting.CustomPath)
	log.Trace("Log path: %s", setting.LogRootPath)

	bucket := c.String("bucket")
	if bucket == "" {
		return fmt.Errorf("missing bucket")
	}

	ctx := context.Background()
	src, err := storage.OpenBucket(ctx, setting.AppWorkPath)
	if err != nil {
		log.Fatal("failed to open source: %v", err)
	}
	defer src.Close()

	setting.BucketURL = bucket
	dst, err := storage.OpenBucket(ctx, "")
	if err != nil {
		log.Fatal("failed to open bucket: %v", err)
	}
	defer dst.Close()

	if c.Bool("local") {
		src, dst = dst, src
	}

	migration := storage.NewMigration(src, dst)
	for _, path := range []string{
		setting.AvatarUploadPath,
		setting.AttachmentPath,
		setting.LFS.ContentPath,
		setting.RepositoryAvatarUploadPath,
	} {
		if filepath.IsAbs(path) {
			log.Trace("Path %s has been in local already, do nothing.", path)
			continue
		}
		migration.SetPrefix(path)
		if err := migration.Run(ctx); err != nil {
			fmt.Printf("failed to migrate from source with prefix %s: %v", path, err)
			return err
		}
	}

	return nil
}
