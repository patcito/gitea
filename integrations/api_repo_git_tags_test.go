// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
)

func TestAPIGitTags(t *testing.T) {
	defer prepareTestEnv(t)()
	user := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)
	// Login as User2.
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session)

	gitRepo, err := git.OpenRepository(repo.RepoPath())
	assert.NoError(t, err)
	defer gitRepo.Close()

	// Set up git config for the tagger
	git.NewCommand("config", "user.name", user.Name).RunInDir(gitRepo.Path)
	git.NewCommand("config", "user.email", user.Email).RunInDir(gitRepo.Path)

	commit, err := gitRepo.GetBranchCommit("master")
	assert.NoError(t, err)
	lTagName := "lightweightTag"
	assert.NoError(t, gitRepo.CreateTag(lTagName, commit.ID.String()))

	aTagName := "annotatedTag"
	aTagMessage := "my annotated message"
	assert.NoError(t, gitRepo.CreateAnnotatedTag(aTagName, aTagMessage, commit.ID.String()))
	aTag, err := gitRepo.GetTag(aTagName)
	assert.NoError(t, err)

	// SHOULD work for annotated tags
	req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/git/tags/%s?token=%s", user.Name, repo.Name, aTag.ID.String(), token)
	res := session.MakeRequest(t, req, http.StatusOK)

	var tag *api.AnnotatedTag
	DecodeJSON(t, res, &tag)

	assert.Equal(t, aTagName, tag.Tag)
	assert.Equal(t, aTag.ID.String(), tag.SHA)
	assert.Equal(t, commit.ID.String(), tag.Object.SHA)
	assert.Equal(t, aTagMessage, tag.Message)
	assert.Equal(t, user.Name, tag.Tagger.Name)
	assert.Equal(t, user.Email, tag.Tagger.Email)
	assert.Equal(t, util.URLJoin(repo.APIURL(), "git/tags", aTag.ID.String()), tag.URL)

	// Should NOT work for lightweight tags
	badReq := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/git/tags/%s?token=%s", user.Name, repo.Name, commit.ID.String(), token)
	session.MakeRequest(t, badReq, http.StatusBadRequest)
}
