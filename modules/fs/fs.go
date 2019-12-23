// Copyright 2019 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package fs

import "github.com/spf13/afero"

// AppFs is an abstract filesystem layer
var AppFs = afero.NewOsFs()
