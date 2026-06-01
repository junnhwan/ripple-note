package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"ripple-note/internal/account"
	"ripple-note/internal/auth"
	"ripple-note/internal/config"
	"ripple-note/internal/interaction"
	"ripple-note/internal/note"
	"ripple-note/internal/outbox"
	"ripple-note/internal/review"
	"ripple-note/internal/storage"
)

const (
	loadTestEmail    = "loadtest@ripple.dev"
	loadTestPassword = "loadtest123"
)

type options struct {
	configPath string
	clean      bool
	users      int
	notes      int
	likes      int
	favorites  int
	comments   int
	follows    int
	tags       int
	batchSize  int
}

func main() {
	opts := parseOptions()

	cfg, err := config.Load(opts.configPath)
	if err != nil {
		log.Fatal("load config: ", err)
	}
	if !cfg.MySQL.Enabled {
		log.Fatal("mysql must be enabled")
	}

	db, err := storage.OpenMySQL(cfg.MySQL)
	if err != nil {
		log.Fatal("connect mysql: ", err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	if err := migrate(db); err != nil {
		log.Fatal("auto migrate: ", err)
	}
	if opts.clean {
		cleanTables(db)
	}

	startedAt := time.Now()
	log.Printf("load seed started: users=%d notes=%d tags=%d likes=%d favorites=%d comments=%d follows=%d batch=%d",
		opts.users, opts.notes, opts.tags, opts.likes, opts.favorites, opts.comments, opts.follows, opts.batchSize)

	passwordHash, err := auth.NewBcryptPasswordHasher().Hash(loadTestPassword)
	if err != nil {
		log.Fatal("hash load test password: ", err)
	}

	userIDs := seedUsers(db, opts, passwordHash)
	tagIDs := seedTags(db, opts)
	noteIDs := seedNotes(db, opts, userIDs)
	seedNoteTags(db, opts, noteIDs, tagIDs)
	seedNoteImages(db, opts, noteIDs)
	seedLikes(db, opts, userIDs, noteIDs)
	seedFavorites(db, opts, userIDs, noteIDs)
	seedComments(db, opts, userIDs, noteIDs)
	seedFollows(db, opts, userIDs)
	refreshNoteCounters(db)

	elapsed := time.Since(startedAt).Round(time.Second)
	fmt.Println("\n========================================")
	fmt.Println("  Load seed complete")
	fmt.Println("========================================")
	fmt.Printf("Dataset: users=%d notes=%d tags=%d likes=%d favorites=%d comments=%d follows=%d\n",
		len(userIDs), len(noteIDs), len(tagIDs), opts.likes, opts.favorites, opts.comments, opts.follows)
	fmt.Printf("Login:   %s / %s\n", loadTestEmail, loadTestPassword)
	fmt.Printf("Elapsed: %s\n", elapsed)
}

func parseOptions() options {
	var opts options
	flag.StringVar(&opts.configPath, "config", "configs/config.local.yaml", "path to YAML config file")
	flag.BoolVar(&opts.clean, "clean", false, "truncate load-test tables before seeding")
	flag.IntVar(&opts.users, "users", 5000, "number of users to create, including the fixed load-test user")
	flag.IntVar(&opts.notes, "notes", 50000, "number of published public notes to create")
	flag.IntVar(&opts.likes, "likes", 300000, "number of note likes to create")
	flag.IntVar(&opts.favorites, "favorites", 100000, "number of note favorites to create")
	flag.IntVar(&opts.comments, "comments", 100000, "number of visible comments to create")
	flag.IntVar(&opts.follows, "follows", 100000, "number of follow relationships to create")
	flag.IntVar(&opts.tags, "tags", 100, "number of tags to create")
	flag.IntVar(&opts.batchSize, "batch-size", 1000, "gorm CreateInBatches size")
	flag.Parse()

	if opts.users < 1 {
		log.Fatal("users must be >= 1")
	}
	if opts.notes < 1 {
		log.Fatal("notes must be >= 1")
	}
	if opts.tags < 1 {
		log.Fatal("tags must be >= 1")
	}
	if opts.batchSize < 1 {
		log.Fatal("batch-size must be >= 1")
	}
	limitPairs("likes", opts.likes, opts.users, opts.notes)
	limitPairs("favorites", opts.favorites, opts.users, opts.notes)
	limitPairs("follows", opts.follows, opts.users, opts.users)
	return opts
}

func limitPairs(name string, requested, left, right int) {
	capacity := left * right
	if name == "follows" {
		capacity = left * (right - 1)
	}
	if requested > capacity {
		log.Fatalf("%s=%d exceeds unique pair capacity=%d", name, requested, capacity)
	}
}

func migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&account.User{},
		&note.Note{},
		&note.NoteImage{},
		&note.Tag{},
		&note.NoteTag{},
		&review.ReviewTask{},
		&review.ReviewTaskEvent{},
		&interaction.NoteLike{},
		&interaction.NoteFavorite{},
		&interaction.Comment{},
		&interaction.Follow{},
		&outbox.Event{},
	)
}

func cleanTables(db *gorm.DB) {
	log.Println("cleaning tables")
	tables := []string{
		"outbox_events", "review_task_events", "review_tasks",
		"note_tags", "note_images",
		"comments", "note_favorites", "note_likes", "follows",
		"notes", "tags", "users",
	}
	db.Exec("SET FOREIGN_KEY_CHECKS = 0")
	for _, table := range tables {
		if err := db.Exec("TRUNCATE TABLE " + table).Error; err != nil {
			log.Fatalf("truncate %s: %v", table, err)
		}
	}
	db.Exec("SET FOREIGN_KEY_CHECKS = 1")
}

func seedUsers(db *gorm.DB, opts options, passwordHash string) []uint64 {
	log.Println("seeding users")
	now := time.Now()
	users := make([]*account.User, 0, opts.users)
	users = append(users, &account.User{
		Email:        loadTestEmail,
		PasswordHash: passwordHash,
		Nickname:     "压测用户",
		AvatarURL:    "https://picsum.photos/seed/loadtest-user/160/160",
		Bio:          "用于 Feed 压测的固定登录账号",
		Role:         account.RoleUser,
		Status:       account.StatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	})

	for i := 1; i < opts.users; i++ {
		users = append(users, &account.User{
			Email:        fmt.Sprintf("load_user_%06d@ripple.dev", i),
			PasswordHash: passwordHash,
			Nickname:     fmt.Sprintf("知涟用户%06d", i),
			AvatarURL:    fmt.Sprintf("https://picsum.photos/seed/ripple-user-%06d/160/160", i),
			Bio:          "由 loadseed 生成的压测用户",
			Role:         account.RoleUser,
			Status:       account.StatusActive,
			CreatedAt:    now.Add(-time.Duration(i%720) * time.Hour),
			UpdatedAt:    now,
		})
	}

	if err := db.Clauses(clause.Insert{Modifier: "IGNORE"}).CreateInBatches(users, opts.batchSize).Error; err != nil {
		log.Fatal("insert users: ", err)
	}

	var ids []uint64
	if err := db.Model(&account.User{}).Where("email LIKE ? OR email = ?", "load_user_%@ripple.dev", loadTestEmail).Order("id ASC").Pluck("id", &ids).Error; err != nil {
		log.Fatal("load user ids: ", err)
	}
	return ids
}

func seedTags(db *gorm.DB, opts options) []uint64 {
	log.Println("seeding tags")
	now := time.Now()
	tags := make([]*note.Tag, 0, opts.tags)
	for i := 0; i < opts.tags; i++ {
		tags = append(tags, &note.Tag{
			Name:      fmt.Sprintf("topic-%03d", i+1),
			CreatedAt: now,
			UpdatedAt: now,
		})
	}
	if err := db.Clauses(clause.Insert{Modifier: "IGNORE"}).CreateInBatches(tags, opts.batchSize).Error; err != nil {
		log.Fatal("insert tags: ", err)
	}

	var ids []uint64
	if err := db.Model(&note.Tag{}).Where("name LIKE ?", "topic-%").Order("id ASC").Pluck("id", &ids).Error; err != nil {
		log.Fatal("load tag ids: ", err)
	}
	return ids
}

func seedNotes(db *gorm.DB, opts options, userIDs []uint64) []uint64 {
	log.Println("seeding notes")
	now := time.Now()
	notes := make([]*note.Note, 0, opts.notes)
	for i := 0; i < opts.notes; i++ {
		authorID := userIDs[(i*17)%len(userIDs)]
		publishedAt := now.Add(-time.Duration(i) * time.Minute)
		hotScore := float64((opts.notes-i)%10000) + float64(i%100)/100
		notes = append(notes, &note.Note{
			AuthorID:    authorID,
			Title:       fmt.Sprintf("知涟压测笔记 %06d：Go 后端 Feed 性能记录", i+1),
			Body:        fmt.Sprintf("这是第 %d 条压测笔记，用于验证游标分页、批量回填、Redis 缓存和 MySQL 索引在 Feed 场景下的表现。", i+1),
			Status:      note.StatusPublished,
			Visibility:  note.VisibilityPublic,
			PublishedAt: &publishedAt,
			HotScore:    hotScore,
			CreatedAt:   publishedAt,
			UpdatedAt:   now,
		})
	}
	if err := db.CreateInBatches(notes, opts.batchSize).Error; err != nil {
		log.Fatal("insert notes: ", err)
	}

	var ids []uint64
	if err := db.Model(&note.Note{}).Where("title LIKE ?", "知涟压测笔记 %").Order("id ASC").Pluck("id", &ids).Error; err != nil {
		log.Fatal("load note ids: ", err)
	}
	return ids
}

func seedNoteTags(db *gorm.DB, opts options, noteIDs, tagIDs []uint64) {
	log.Println("seeding note tags")
	noteTags := make([]*note.NoteTag, 0, len(noteIDs)*2)
	for i, noteID := range noteIDs {
		noteTags = append(noteTags, &note.NoteTag{NoteID: noteID, TagID: tagIDs[i%len(tagIDs)]})
		if len(tagIDs) > 1 && i%3 != 0 {
			noteTags = append(noteTags, &note.NoteTag{NoteID: noteID, TagID: tagIDs[(i*7+3)%len(tagIDs)]})
		}
	}
	if err := db.Clauses(clause.Insert{Modifier: "IGNORE"}).CreateInBatches(noteTags, opts.batchSize).Error; err != nil {
		log.Fatal("insert note tags: ", err)
	}
	if err := db.Exec(`
		UPDATE tags t
		LEFT JOIN (
			SELECT tag_id, COUNT(*) AS total
			FROM note_tags
			GROUP BY tag_id
		) nt ON nt.tag_id = t.id
		SET t.notes_count = COALESCE(nt.total, 0)
		WHERE t.name LIKE 'topic-%'
	`).Error; err != nil {
		log.Fatal("refresh tag counters: ", err)
	}
}

func seedNoteImages(db *gorm.DB, opts options, noteIDs []uint64) {
	log.Println("seeding note images")
	images := make([]*note.NoteImage, 0, len(noteIDs)/2)
	now := time.Now()
	for i, noteID := range noteIDs {
		if i%3 == 0 {
			images = append(images, &note.NoteImage{
				NoteID:     noteID,
				URL:        fmt.Sprintf("https://picsum.photos/seed/ripple-note-%06d/900/600", i+1),
				StorageKey: fmt.Sprintf("loadseed_%06d", i+1),
				SortOrder:  1,
				CreatedAt:  now,
			})
		}
	}
	if len(images) == 0 {
		return
	}
	if err := db.CreateInBatches(images, opts.batchSize).Error; err != nil {
		log.Fatal("insert note images: ", err)
	}
}

func seedLikes(db *gorm.DB, opts options, userIDs, noteIDs []uint64) {
	log.Println("seeding likes")
	pairs := generatePairs(opts.likes, len(userIDs), len(noteIDs), false)
	now := time.Now()
	likes := make([]*interaction.NoteLike, 0, opts.batchSize)
	for _, pair := range pairs {
		likes = append(likes, &interaction.NoteLike{
			UserID:    userIDs[pair.left],
			NoteID:    noteIDs[pair.right],
			CreatedAt: now.Add(-time.Duration(pair.left%168) * time.Hour),
		})
		if len(likes) >= opts.batchSize {
			insertLikes(db, likes)
			likes = likes[:0]
		}
	}
	insertLikes(db, likes)
}

func seedFavorites(db *gorm.DB, opts options, userIDs, noteIDs []uint64) {
	log.Println("seeding favorites")
	pairs := generatePairs(opts.favorites, len(userIDs), len(noteIDs), false)
	now := time.Now()
	favorites := make([]*interaction.NoteFavorite, 0, opts.batchSize)
	for _, pair := range pairs {
		favorites = append(favorites, &interaction.NoteFavorite{
			UserID:    userIDs[(pair.left*13+7)%len(userIDs)],
			NoteID:    noteIDs[(pair.right*19+11)%len(noteIDs)],
			CreatedAt: now.Add(-time.Duration(pair.right%168) * time.Hour),
		})
		if len(favorites) >= opts.batchSize {
			insertFavorites(db, favorites)
			favorites = favorites[:0]
		}
	}
	insertFavorites(db, favorites)
}

func seedComments(db *gorm.DB, opts options, userIDs, noteIDs []uint64) {
	log.Println("seeding comments")
	now := time.Now()
	comments := make([]*interaction.Comment, 0, opts.batchSize)
	for i := 0; i < opts.comments; i++ {
		comments = append(comments, &interaction.Comment{
			NoteID:    noteIDs[(i*31+7)%len(noteIDs)],
			AuthorID:  userIDs[(i*17+3)%len(userIDs)],
			Body:      fmt.Sprintf("压测评论 %06d：Feed 展示时不拉取评论正文，只统计 comments_count。", i+1),
			Status:    interaction.CommentStatusVisible,
			CreatedAt: now.Add(-time.Duration(i%10080) * time.Minute),
		})
		if len(comments) >= opts.batchSize {
			if err := db.CreateInBatches(comments, opts.batchSize).Error; err != nil {
				log.Fatal("insert comments: ", err)
			}
			comments = comments[:0]
		}
	}
	if len(comments) > 0 {
		if err := db.CreateInBatches(comments, opts.batchSize).Error; err != nil {
			log.Fatal("insert comments: ", err)
		}
	}
}

func seedFollows(db *gorm.DB, opts options, userIDs []uint64) {
	log.Println("seeding follows")
	pairs := generatePairs(opts.follows, len(userIDs), len(userIDs), true)
	now := time.Now()
	follows := make([]*interaction.Follow, 0, opts.batchSize)
	for _, pair := range pairs {
		follows = append(follows, &interaction.Follow{
			FollowerID: userIDs[pair.left],
			FolloweeID: userIDs[pair.right],
			CreatedAt:  now.Add(-time.Duration(pair.left%720) * time.Hour),
		})
		if len(follows) >= opts.batchSize {
			insertFollows(db, follows)
			follows = follows[:0]
		}
	}
	insertFollows(db, follows)
}

func insertLikes(db *gorm.DB, likes []*interaction.NoteLike) {
	if len(likes) == 0 {
		return
	}
	if err := db.Clauses(clause.Insert{Modifier: "IGNORE"}).CreateInBatches(likes, len(likes)).Error; err != nil {
		log.Fatal("insert likes: ", err)
	}
}

func insertFavorites(db *gorm.DB, favorites []*interaction.NoteFavorite) {
	if len(favorites) == 0 {
		return
	}
	if err := db.Clauses(clause.Insert{Modifier: "IGNORE"}).CreateInBatches(favorites, len(favorites)).Error; err != nil {
		log.Fatal("insert favorites: ", err)
	}
}

func insertFollows(db *gorm.DB, follows []*interaction.Follow) {
	if len(follows) == 0 {
		return
	}
	if err := db.Clauses(clause.Insert{Modifier: "IGNORE"}).CreateInBatches(follows, len(follows)).Error; err != nil {
		log.Fatal("insert follows: ", err)
	}
}

func refreshNoteCounters(db *gorm.DB) {
	log.Println("refreshing note counters")
	if err := db.Exec(`
		UPDATE notes n
		LEFT JOIN (
			SELECT note_id, COUNT(*) AS total
			FROM note_likes
			WHERE deleted_at IS NULL
			GROUP BY note_id
		) l ON l.note_id = n.id
		SET n.likes_count = COALESCE(l.total, 0)
		WHERE n.title LIKE '知涟压测笔记 %'
	`).Error; err != nil {
		log.Fatal("refresh likes_count: ", err)
	}
	if err := db.Exec(`
		UPDATE notes n
		LEFT JOIN (
			SELECT note_id, COUNT(*) AS total
			FROM note_favorites
			WHERE deleted_at IS NULL
			GROUP BY note_id
		) f ON f.note_id = n.id
		SET n.favorites_count = COALESCE(f.total, 0)
		WHERE n.title LIKE '知涟压测笔记 %'
	`).Error; err != nil {
		log.Fatal("refresh favorites_count: ", err)
	}
	if err := db.Exec(`
		UPDATE notes n
		LEFT JOIN (
			SELECT note_id, COUNT(*) AS total
			FROM comments
			WHERE deleted_at IS NULL AND status = 'visible'
			GROUP BY note_id
		) c ON c.note_id = n.id
		SET n.comments_count = COALESCE(c.total, 0)
		WHERE n.title LIKE '知涟压测笔记 %'
	`).Error; err != nil {
		log.Fatal("refresh comments_count: ", err)
	}
}

type pair struct {
	left  int
	right int
}

func generatePairs(total, leftCount, rightCount int, excludeSame bool) []pair {
	if total == 0 {
		return nil
	}
	if leftCount <= 0 || rightCount <= 0 {
		log.Fatal("pair dimensions must be positive")
	}

	pairs := make([]pair, 0, total)
	seen := make(map[uint64]struct{}, total)
	for i := 0; len(pairs) < total; i++ {
		left := spread(i, leftCount, 2654435761)
		right := spread(i, rightCount, 2246822519)
		if excludeSame && left == right {
			right = (right + 1 + i%(rightCount-1)) % rightCount
		}
		key := uint64(uint32(left))<<32 | uint64(uint32(right))
		if _, ok := seen[key]; ok {
			left = spread(i+len(pairs)+17, leftCount, 3266489917)
			right = spread(i+len(pairs)+31, rightCount, 668265263)
			if excludeSame && left == right {
				right = (right + 1) % rightCount
			}
			key = uint64(uint32(left))<<32 | uint64(uint32(right))
			if _, ok := seen[key]; ok {
				continue
			}
		}
		seen[key] = struct{}{}
		pairs = append(pairs, pair{left: left, right: right})
		if i > int(math.MaxInt32)/2 {
			log.Fatalf("could not generate %d unique pairs", total)
		}
	}
	return pairs
}

func spread(i, mod, multiplier int) int {
	if mod == 1 {
		return 0
	}
	value := uint64(i+1) * uint64(multiplier)
	value ^= value >> 16
	return int(value % uint64(mod))
}
