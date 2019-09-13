package libgin

import (
	"time"

	"github.com/G-Node/gig"
)

// NOTE: TEMPORARY COPY FROM gin-dex

type SearchRequest struct {
	Token  string
	CsrfT  string
	UserID int64
	Query  string
	SType  int64
}

const (
	SEARCH_MATCH = iota
	SEARCH_FUZZY
	SEARCH_WILDCARD
	SEARCH_QUERY
	SEARCH_SUGGEST
)

type BlobSResult struct {
	Source    *IndexBlob  `json:"_source"`
	Score     float64     `json:"_score"`
	Highlight interface{} `json:"highlight"`
}

type CommitSResult struct {
	Source    *IndexCommit `json:"_source"`
	Score     float64      `json:"_score"`
	Highlight interface{}  `json:"highlight"`
}

type SearchResults struct {
	Blobs   []BlobSResult
	Commits []CommitSResult
}

type IndexBlob struct {
	*gig.Blob
	GinRepoName  string
	GinRepoId    string
	FirstCommit  string
	Id           int64
	Oid          gig.SHA1
	IndexingTime time.Time
	Content      string
	Path         string
}

type IndexCommit struct {
	*gig.Commit
	GinRepoId    string
	Oid          gig.SHA1
	GinRepoName  string
	IndexingTime time.Time
}

// StartIndexing sends an indexing request to the configured indexing service
// for a repository.
// func StartIndexing(user, owner *gogs.User, repo *gogs.Repository) {
// 	if !setting.Search.Do {
// 		return
// 	}
// 	var ireq struct{ RepoID, RepoPath string }
// 	ireq.RepoID = fmt.Sprintf("%d", repo.ID)
// 	ireq.RepoPath = repo.FullName()
// 	data, err := json.Marshal(ireq)
// 	if err != nil {
// 		return
// 	}
// 	req, _ := http.NewRequest(http.MethodPost, setting.Search.IndexURL, bytes.NewReader(data))
// 	client := http.Client{}
// 	resp, err := client.Do(req)
// 	if err != nil || resp.StatusCode != http.StatusOK {
// 		return
// 	}
// }
