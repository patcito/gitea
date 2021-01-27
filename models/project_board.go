// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/builder"
	"xorm.io/xorm"
)

type (
	// ProjectBoardType is used to represent a project board type
	ProjectBoardType uint8

	// ProjectBoardList is a list of all project boards in a repository
	ProjectBoardList []*ProjectBoard
)

const (
	// ProjectBoardTypeNone is a project board type that has no predefined columns
	ProjectBoardTypeNone ProjectBoardType = iota

	// ProjectBoardTypeBasicKanban is a project board type that has basic predefined columns
	ProjectBoardTypeBasicKanban

	// ProjectBoardTypeBugTriage is a project board type that has predefined columns suited to hunting down bugs
	ProjectBoardTypeBugTriage
)

// ProjectBoard is used to represent boards on a project
type ProjectBoard struct {
	ID       int64 `xorm:"pk autoincr"`
	Title    string
	Default  bool `xorm:"NOT NULL DEFAULT false"` // issues not assigned to a specific board will be assigned to this board
	Priority int  `xorm:"NOT NULL DEFAULT 0"`

	ProjectID int64 `xorm:"INDEX NOT NULL"`
	CreatorID int64 `xorm:"NOT NULL"`

	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`

	Issues        []*Issue        `xorm:"-"`
	ProjectIssues []*ProjectIssue `xorm:"-"`
}

// IsProjectBoardTypeValid checks if the project board type is valid
func IsProjectBoardTypeValid(p ProjectBoardType) bool {
	switch p {
	case ProjectBoardTypeNone, ProjectBoardTypeBasicKanban, ProjectBoardTypeBugTriage:
		return true
	default:
		return false
	}
}

func createBoardsForProjectsType(sess *xorm.Session, project *Project) error {

	var items []string

	switch project.BoardType {

	case ProjectBoardTypeBugTriage:
		items = setting.Project.ProjectBoardBugTriageType

	case ProjectBoardTypeBasicKanban:
		items = setting.Project.ProjectBoardBasicKanbanType

	case ProjectBoardTypeNone:
		fallthrough
	default:
		return nil
	}

	if len(items) == 0 {
		return nil
	}

	var boards = make([]ProjectBoard, 0, len(items))

	for _, v := range items {
		boards = append(boards, ProjectBoard{
			CreatedUnix: timeutil.TimeStampNow(),
			CreatorID:   project.CreatorID,
			Title:       v,
			ProjectID:   project.ID,
		})
	}

	_, err := sess.Insert(boards)
	return err
}

// NewProjectBoard adds a new project board to a given project
func NewProjectBoard(board *ProjectBoard) error {
	_, err := x.Insert(board)
	return err
}

// DeleteProjectBoardByID removes all issues references to the project board.
func DeleteProjectBoardByID(boardID int64) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if err := deleteProjectBoardByID(sess, boardID); err != nil {
		return err
	}

	return sess.Commit()
}

func deleteProjectBoardByID(e Engine, boardID int64) error {
	board, err := getProjectBoard(e, boardID)
	if err != nil {
		if IsErrProjectBoardNotExist(err) {
			return nil
		}

		return err
	}

	if err = board.removeIssues(e); err != nil {
		return err
	}

	if _, err := e.ID(board.ID).Delete(board); err != nil {
		return err
	}
	return nil
}

func deleteProjectBoardByProjectID(e Engine, projectID int64) error {
	_, err := e.Where("project_id=?", projectID).Delete(&ProjectBoard{})
	return err
}

// GetProjectBoard fetches the current board of a project
func GetProjectBoard(boardID int64) (*ProjectBoard, error) {
	return getProjectBoard(x, boardID)
}

func getProjectBoard(e Engine, boardID int64) (*ProjectBoard, error) {
	board := new(ProjectBoard)

	has, err := e.ID(boardID).Get(board)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrProjectBoardNotExist{BoardID: boardID}
	}

	return board, nil
}

// UpdateProjectBoard updates the title of a project board
func UpdateProjectBoard(board *ProjectBoard) error {
	return updateProjectBoard(x, board)
}

func updateProjectBoard(e Engine, board *ProjectBoard) error {
	_, err := e.ID(board.ID).Cols(
		"title",
	).Update(board)
	return err
}

// GetProjectBoards fetches all boards related to a project
// if no default board set, first board is a temporary "Uncategorized" board
func GetProjectBoards(projectID int64) (ProjectBoardList, error) {
	return getProjectBoards(x, projectID)
}

func getProjectBoards(e Engine, projectID int64) ([]*ProjectBoard, error) {
	var boards = make([]*ProjectBoard, 0, 5)

	if err := e.Where("project_id=? AND `default`=?", projectID, false).OrderBy("priority").Find(&boards); err != nil {
		return nil, err
	}

	defaultB, err := getDefaultBoard(e, projectID)
	if err != nil {
		return nil, err
	}

	return append([]*ProjectBoard{defaultB}, boards...), nil
}

// getDefaultBoard return default board and create a dummy if none exist
func getDefaultBoard(e Engine, projectID int64) (*ProjectBoard, error) {
	var board ProjectBoard
	exist, err := e.Where("project_id=? AND `default`=?", projectID, true).Get(&board)
	if err != nil {
		return nil, err
	}
	if exist {
		return &board, nil
	}

	// represents a board for issues not assigned to one
	return &ProjectBoard{
		ProjectID: projectID,
		Title:     "Uncategorized",
		Default:   true,
	}, nil
}

// SetDefaultBoard represents a board for issues not assigned to one
// if boardID is 0 unset default
func SetDefaultBoard(projectID, boardID int64) error {
	sess := x

	_, err := sess.Where(builder.Eq{
		"project_id": projectID,
		"`default`":  true,
	}).Cols("`default`").Update(&ProjectBoard{Default: false})
	if err != nil {
		return err
	}

	if boardID > 0 {
		_, err = sess.ID(boardID).Where(builder.Eq{"project_id": projectID}).
			Cols("`default`").Update(&ProjectBoard{Default: true})
	}

	return err
}

// LoadProjectIssues load project issues assigned to this board
func (b *ProjectBoard) LoadProjectIssues() ([]*ProjectIssue, error) {
	var boardID int64
	if !b.Default {
		boardID = b.ID

	} else {
		// Issues without ProjectBoardID
		boardID = 0
	}
	var projectIssues []*ProjectIssue
	var issues []*Issue
	if err := x.Table("issue").
		Cols("issue.id, issue.repo_id, issue.index, issue.poster_id,issue.name,issue.milestone_id,issue.is_closed,issue.is_pull,issue.created_unix,issue.updated_unix, project_issue.id as project_issue_id, project_issue.project_id as project_issue_project_id, project_issue.project_board_id as project_issue_project_board_id").
		Join("INNER", "project_issue", "issue.id = project_issue.issue_id").
		Where("project_board_id = ? and project_id =?",
			boardID, b.ProjectID).
		OrderBy("`project_issue`.priority").Find(&issues); err != nil {
		log.Error("LoadAttributes: %v", err)
	}

	if err := IssueList(issues).LoadAttributes(); err != nil {
		log.Error("LoadAttributes: %v", err)
	}

	for _, issue := range issues {
		projectIssue := issue.ProjectIssueID
		projectIssue.Issue = issue
		projectIssues = append(projectIssues, &projectIssue)
	}
	return projectIssues, nil
}

// LoadIssues load issues assigned to the boards
func (bs ProjectBoardList) LoadIssues() error {
	for i := range bs {
		il, err := bs[i].LoadProjectIssues()
		if err != nil {
			return err
		}
		bs[i].ProjectIssues = il
	}
	return nil
}

// UpdateBoardsPriority updates boards priority for a project
func UpdateBoardsPriority(boards []ProjectBoard) error {
	sess := x.NewSession()
	if err := sess.Begin(); err != nil {
		return err
	}
	defer sess.Close()
	for _, board := range boards {
		if _, err := sess.ID(board.ID).Cols("priority").Update(&board); err != nil {
			log.Info("failed updating board priorities %s", err)
			return err
		}
	}
	return sess.Commit()
}
