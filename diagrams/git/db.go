package git

import (
	"slices"
	"strconv"
	"strings"

	"github.com/Broderick-Westrope/pele-go/internal/jsobj"
	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
	"github.com/Broderick-Westrope/pele-go/internal/omap"
	"github.com/Broderick-Westrope/pele-go/internal/perr"
)

// Commit types, as upstream's commitType.
const (
	commitNormal = iota
	commitReverse
	commitHighlight
	commitMerge
	commitCherryPick
)

type gitCommit struct {
	id      string
	message string
	seq     int
	typ     int
	tags    []string
	parents []string
	branch  string
	// customType is nil where upstream's is undefined.
	customType *int
	customID   bool
}

type gitModel struct {
	// Empty strings stand for undefined: every use treats the two alike.
	title, accTitle, accDescr string
	direction                 string
	// Keyed by commit id, in the order the ids first appeared.
	commits *omap.Map[string, *gitCommit]
	// The id of each branch's latest commit, or "" while the branch has none.
	branches *omap.Map[string, string]
	// Branch names in drawing order: by `order`, then by creation.
	lanes         []string
	currentBranch string
	head          *gitCommit
	warnings      []string
}

func fail(message string) {
	e := perr.New(message, perr.Semantic)
	e.Type = "gitGraph"
	perr.Throw(e)
}

func typeCode(name string) int {
	switch name {
	case "REVERSE":
		return commitReverse
	case "HIGHLIGHT":
		return commitHighlight
	}
	return commitNormal
}

// gitDb is upstream's GitDbParseProvider as buildGit implements it.
type gitDb struct {
	commits  *omap.Map[string, *gitCommit]
	branches *omap.Map[string, string]
	// orders is upstream's Map of branch orders; every value is a number.
	orders    *omap.Map[string, float64]
	warnings  []string
	head      *gitCommit
	current   string
	direction string
	seq       int
}

func (db *gitDb) populate(a gitGraphAst) {
	if a.dir != "" {
		db.setDirection(a.dir)
	}
	for _, statement := range a.statements {
		db.parseStatement(statement)
	}
}

func (db *gitDb) parseStatement(statement statementAst) {
	switch statement.kind {
	case stCommit:
		msg := ""
		if statement.message != nil {
			msg = *statement.message
		}
		typ := commitNormal
		if statement.typ != nil {
			typ = typeCode(*statement.typ)
		}
		db.commit(msg, statement.id, typ, statement.tags)
	case stBranch:
		order := 0.0
		if statement.order != nil {
			order = *statement.order
		}
		db.branch(statement.name, order)
	case stMerge:
		id := ""
		if statement.id != nil {
			id = *statement.id
		}
		var typ *int
		if statement.typ != nil {
			typ = ptr(typeCode(*statement.typ))
		}
		db.merge(statement.name, id, typ, statement.tags)
	case stCheckout:
		db.checkout(statement.name)
	case stCherryPicking:
		tags := statement.tags
		if len(tags) == 0 {
			tags = nil
		}
		db.cherryPick(statement.id, "", tags, statement.parent)
	}
}

func (db *gitDb) setDirection(dir string) {
	db.direction = dir
}

// nextID makes a commit id. Mermaid appends seven random hex digits to the sequence number.
// These digits are derived from the sequence number instead, so the same text always gives
// the same ids.
func (db *gitDb) nextID() string {
	for salt := float64(db.seq + 1); ; salt += 0x9e3779b1 {
		hash := uint32(uint64(salt)) * 0x9e3779b1
		hex := strconv.FormatUint(uint64(hash), 16)
		hex = strings.Repeat("0", 8-len(hex)) + hex
		id := strconv.Itoa(db.seq) + "-" + hex[:7]
		if !db.commits.Has(id) {
			return id
		}
	}
}

func (db *gitDb) add(commit *gitCommit) {
	db.head = commit
	db.commits.Set(commit.id, commit)
	db.branches.Set(db.current, commit.id)
}

func (db *gitDb) checkout(branch string) {
	if !db.branches.Has(branch) {
		fail(`Trying to checkout branch which is not yet created. (Help try using "branch ` + branch + `")`)
	}
	db.current = branch
	id, _ := db.branches.Get(branch)
	db.head = nil
	if id != "" {
		if c, ok := db.commits.Get(id); ok {
			db.head = c
		}
	}
}

func (db *gitDb) headParents(others ...string) []string {
	if db.head == nil {
		return []string{}
	}
	return append([]string{db.head.id}, others...)
}

func (db *gitDb) commit(msg string, id *string, typ int, tags []string) {
	commitID := ""
	if id != nil {
		commitID = *id
	}
	if commitID == "" {
		commitID = db.nextID()
	}
	commit := &gitCommit{
		id:      commitID,
		message: msg,
		seq:     db.seq,
		typ:     typ,
		tags:    slices.Clone(tags),
		parents: db.headParents(),
		branch:  db.current,
	}
	db.seq++
	if commit.tags == nil {
		commit.tags = []string{}
	}
	if db.commits.Has(commit.id) {
		db.warnings = append(db.warnings, "Commit ID "+commit.id+" already exists")
	}
	db.add(commit)
}

func (db *gitDb) branch(name string, order float64) {
	if db.branches.Has(name) {
		fail(`Trying to create an existing branch. (Help: Either use a new name if you want create a new branch or try using "checkout ` + name + `")`)
	}
	head := ""
	if db.head != nil {
		head = db.head.id
	}
	db.branches.Set(name, head)
	db.orders.Set(name, order)
	db.checkout(name)
}

func (db *gitDb) merge(other, customID string, typ *int, tags []string) {
	current := db.current
	currentHead, _ := db.branches.Get(current)
	otherHead, _ := db.branches.Get(other)
	var currentCommit, otherCommit *gitCommit
	if currentHead != "" {
		currentCommit, _ = db.commits.Get(currentHead)
	}
	if otherHead != "" {
		otherCommit, _ = db.commits.Get(otherHead)
	}
	if currentCommit != nil && otherCommit != nil && currentCommit.branch == other {
		fail("Cannot merge branch '" + other + "' into itself.")
	}
	if current == other {
		fail(`Incorrect usage of "merge". Cannot merge a branch to itself`)
	}
	if currentCommit == nil {
		fail(`Incorrect usage of "merge". Current branch (` + current + `)has no commits`)
	}
	if !db.branches.Has(other) {
		fail(`Incorrect usage of "merge". Branch to be merged (` + other + `) does not exist`)
	}
	if otherCommit == nil {
		fail(`Incorrect usage of "merge". Branch to be merged (` + other + `) has no commits`)
	}
	if currentCommit == otherCommit {
		fail(`Incorrect usage of "merge". Both branches have same head`)
	}
	if customID != "" && db.commits.Has(customID) {
		fail(`Incorrect usage of "merge". Commit with id:` + customID + ` already exists, use different custom id`)
	}
	id := customID
	if id == "" {
		id = db.nextID()
	}
	commit := &gitCommit{
		id:         id,
		message:    "merged branch " + other + " into " + current,
		seq:        db.seq,
		parents:    db.headParents(otherHead),
		branch:     current,
		typ:        commitMerge,
		customType: typ,
		customID:   customID != "",
		tags:       slices.Clone(tags),
	}
	db.seq++
	if commit.tags == nil {
		commit.tags = []string{}
	}
	db.add(commit)
}

func (db *gitDb) cherryPick(sourceID *string, targetID string, tags []string, parent *string) {
	var source *gitCommit
	if sourceID != nil && *sourceID != "" {
		source, _ = db.commits.Get(*sourceID)
	}
	if source == nil {
		fail(`Incorrect usage of "cherryPick". Source commit id should exist and provided`)
	}
	if parent != nil && *parent != "" && !slices.Contains(source.parents, *parent) {
		fail("Invalid operation: The specified parent commit is not an immediate parent of the cherry-picked commit.")
	}
	if source.typ == commitMerge && (parent == nil || *parent == "") {
		fail("Incorrect usage of cherry-pick: If the source commit is a merge commit, an immediate parent commit must be specified.")
	}
	if targetID != "" && db.commits.Has(targetID) {
		return
	}
	if source.branch == db.current {
		fail(`Incorrect usage of "cherryPick". Source commit is already on current branch`)
	}
	currentHead, _ := db.branches.Get(db.current)
	if currentHead == "" || !db.commits.Has(currentHead) {
		fail(`Incorrect usage of "cherry-pick". Current branch (` + db.current + `)has no commits`)
	}
	var picked []string
	if tags != nil {
		picked = []string{}
		for _, tag := range tags {
			if tag != "" {
				picked = append(picked, tag)
			}
		}
	} else {
		tag := "cherry-pick:" + source.id
		if source.typ == commitMerge {
			// parent is set here: a merge without one failed above.
			tag += "|parent:" + *parent
		}
		picked = []string{tag}
	}
	commit := &gitCommit{
		id:      db.nextID(),
		message: "cherry-picked " + source.message + " into " + db.current,
		seq:     db.seq,
		parents: db.headParents(source.id),
		branch:  db.current,
		typ:     commitCherryPick,
		tags:    picked,
	}
	db.seq++
	db.add(commit)
}

func buildGit(a gitGraphAst, config *jsobj.Object, title *string) *gitModel {
	var git *jsobj.Object
	if v, ok := config.Get("gitGraph"); ok {
		git, _ = v.(*jsobj.Object)
	}
	mainName := "main"
	if v, ok := git.Get("mainBranchName"); ok {
		if s, ok := v.(string); ok {
			mainName = s
		}
	}
	mainOrder := 0.0
	if v, ok := git.Get("mainBranchOrder"); ok {
		if n, ok := v.(float64); ok {
			mainOrder = n
		}
	}

	db := &gitDb{
		commits:   &omap.Map[string, *gitCommit]{},
		branches:  &omap.Map[string, string]{},
		orders:    &omap.Map[string, float64]{},
		warnings:  []string{},
		current:   mainName,
		direction: "LR",
	}
	db.branches.Set(mainName, "")
	db.orders.Set(mainName, mainOrder)
	db.populate(a)

	// A branch made without an order sorts by creation among those of order zero, as in Mermaid.
	type lane struct {
		name  string
		order float64
	}
	var sorted []lane
	for name, order := range db.orders.All() {
		sorted = append(sorted, lane{name, order})
	}
	jsSort(sorted, func(a, b lane) float64 { return a.order - b.order })
	lanes := make([]string, len(sorted))
	for i, l := range sorted {
		lanes[i] = l.name
	}

	m := &gitModel{
		direction:     db.direction,
		commits:       db.commits,
		branches:      db.branches,
		lanes:         lanes,
		currentBranch: db.current,
		head:          db.head,
		warnings:      db.warnings,
	}
	if a.title != nil && *a.title != "" {
		m.title = *a.title
	} else if title != nil {
		m.title = *title
	}
	if a.accTitle != nil && *a.accTitle != "" {
		m.accTitle = jsstr.TrimStart(*a.accTitle)
	}
	if a.accDescr != nil && *a.accDescr != "" {
		m.accDescr = collapseIndentedBreaks(*a.accDescr)
	}
	return m
}

// jsSort is V8's Array.prototype.sort for a comparator that can return NaN, which no
// consistent order can stand for. Below 64 elements V8 sorts one run by binary insertion;
// that is reproduced here. Longer arrays merge runs, and are sorted stably instead, which
// gives the same order whenever the comparator never returns NaN.
func jsSort[T any](items []T, compare func(a, b T) float64) {
	n := len(items)
	if n < 2 {
		return
	}
	if n >= 64 {
		slices.SortStableFunc(items, func(a, b T) int {
			switch d := compare(a, b); {
			case d < 0:
				return -1
			case d > 0:
				return 1
			}
			return 0
		})
		return
	}
	// CountAndMakeRun.
	run := 2
	descending := compare(items[1], items[0]) < 0
	for i := 2; i < n; i++ {
		order := compare(items[i], items[i-1])
		if descending && order >= 0 || !descending && order < 0 {
			break
		}
		run++
	}
	if descending {
		slices.Reverse(items[:run])
	}
	// BinaryInsertionSort.
	for start := run; start < n; start++ {
		left, right := 0, start
		pivot := items[start]
		for left < right {
			mid := left + (right-left)>>1
			if compare(pivot, items[mid]) < 0 {
				right = mid
			} else {
				left = mid + 1
			}
		}
		copy(items[left+1:start+1], items[left:start])
		items[left] = pivot
	}
}

// collapseIndentedBreaks is s.replace(/\n\s+/g, '\n').
func collapseIndentedBreaks(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] != '\n' {
			b.WriteByte(s[i])
			i++
			continue
		}
		j := i + 1
		for j < len(s) {
			r, n := jsstr.Decode(s, j)
			if !jsstr.IsSpace(r) {
				break
			}
			j += n
		}
		b.WriteByte('\n')
		i = j
	}
	return b.String()
}
