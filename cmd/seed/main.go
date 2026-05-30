package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"ripple-note/internal/account"
	"ripple-note/internal/auth"
	"ripple-note/internal/config"
	"ripple-note/internal/interaction"
	"ripple-note/internal/note"
	"ripple-note/internal/outbox"
	"ripple-note/internal/review"
	"ripple-note/internal/storage"

	"gorm.io/gorm"
)

func main() {
	configPath := flag.String("config", "configs/config.local.yaml", "path to YAML config file")
	clean := flag.Bool("clean", false, "truncate all tables before seeding")
	flag.Parse()

	cfg, err := config.Load(*configPath)
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

	if err := db.AutoMigrate(
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
	); err != nil {
		log.Fatal("auto migrate: ", err)
	}

	if *clean {
		log.Println("Cleaning all tables...")
		tables := []string{
			"outbox_events", "review_task_events", "review_tasks",
			"note_tags", "note_images", "notes", "tags",
			"comments", "note_favorites", "note_likes", "follows",
			"users",
		}
		db.Exec("SET FOREIGN_KEY_CHECKS = 0")
		for _, t := range tables {
			if err := db.Exec("TRUNCATE TABLE " + t).Error; err != nil {
				log.Fatalf("truncate %s: %v", t, err)
			}
		}
		db.Exec("SET FOREIGN_KEY_CHECKS = 1")
		log.Println("Tables cleaned.")
	}

	hasher := auth.NewBcryptPasswordHasher()

	log.Println("Creating users...")
	createUser(db, hasher, "admin@ripple.dev", "admin123", "知涟管理员", "admin", "我负责审核平台内容，确保社区氛围健康。")
	user2 := createUser(db, hasher, "alice@ripple.dev", "alice123", "Alice", "user", "Go 后端工程师 | 热爱开源和技术写作")
	user3 := createUser(db, hasher, "bob@ripple.dev", "bob123", "Bob", "user", "全栈开发者 | React + Go")
	user4 := createUser(db, hasher, "carol@ripple.dev", "carol123", "Carol", "user", "产品设计师 | 关注用户体验")

	log.Println("Creating tags...")
	tagGo := createTag(db, "go")
	tagBackend := createTag(db, "backend")
	tagReact := createTag(db, "react")
	tagTypescript := createTag(db, "typescript")
	tagDesign := createTag(db, "设计")
	tagCareer := createTag(db, "职场")
	tagTutorial := createTag(db, "教程")
	tagMicroservice := createTag(db, "微服务")
	tagRedis := createTag(db, "redis")
	tagDocker := createTag(db, "docker")

	log.Println("Creating notes...")
	now := time.Now()

	type noteData struct {
		title, body string
		authorID    uint64
		status      string
		publishedAt *time.Time
		hotScore    float64
		likes, favorites, comments uint64
		imageURLs   []string
		tagIDs      []uint64
	}

	notes := []noteData{
		{
			title: "Go 1.22 泛型实战：如何用类型参数重构项目",
			body: "泛型在 Go 1.18 引入后，终于在 1.22 变得更加实用。本文分享我在实际项目中用泛型重构的一些经验。\n\n## 为什么要用泛型？\n\n项目中到处都是 interface{} 和类型断言，这不仅降低了类型安全性，还让代码难以维护。\n\n## 实战案例\n\n最典型的场景是通用分页函数。之前每个 model 都要写一套分页逻辑，现在可以用泛型统一。\n\n## 总结\n\n泛型不是银弹，但在工具函数、集合操作等场景下确实能显著减少重复代码。建议从这些场景开始尝试。",
			authorID: user2.ID, status: note.StatusPublished,
			publishedAt: timePtr(now.Add(-48 * time.Hour)), hotScore: 87.5,
			likes: 24, favorites: 8, comments: 6,
			imageURLs: []string{"https://picsum.photos/seed/go-generic/800/600", "https://picsum.photos/seed/go-code/800/600"},
			tagIDs:    []uint64{tagGo.ID, tagBackend.ID, tagTutorial.ID},
		},
		{
			title: "从零搭建微服务：用 Gin + GORM 构建内容平台",
			body: "这篇文章记录了我用 Gin + GORM 从零搭建一个内容社区后端的完整过程。\n\n## 技术选型\n\nWeb 框架：Gin — 性能优秀，中间件生态丰富\nORM：GORM — Go 生态最成熟的 ORM\n数据库：MySQL 8.0\n缓存：Redis\n消息队列：RabbitMQ\n\n## 关键设计决策\n\n1. 游标分页：Feed 场景必须用游标分页，避免 offset 在数据变更时出现跳页\n2. Outbox 模式：业务写入和事件发布在同一个事务中，保证最终一致性\n3. 幂等设计：点赞、关注等操作通过软删除+唯一索引实现天然幂等",
			authorID: user2.ID, status: note.StatusPublished,
			publishedAt: timePtr(now.Add(-36 * time.Hour)), hotScore: 92.3,
			likes: 35, favorites: 12, comments: 9,
			imageURLs: []string{"https://picsum.photos/seed/microservice/800/600"},
			tagIDs:    []uint64{tagGo.ID, tagBackend.ID, tagMicroservice.ID},
		},
		{
			title: "React 19 新特性一览：Server Components 落地实践",
			body: "React 19 终于将 Server Components 正式纳入稳定版。本文总结核心变化和实战经验。\n\n## Server Components\n\n最大的变化是组件可以默认在服务端渲染，只有标记了 use client 的组件才会在客户端执行。\n\n好处：\n- 减少客户端 bundle 体积\n- 可以直接访问数据库\n- SEO 更友好\n\n## 性能对比\n\n在相同数据量下，FCP 从 1.8s 降到 0.6s，TTI 从 3.2s 降到 1.1s。效果显著。",
			authorID: user3.ID, status: note.StatusPublished,
			publishedAt: timePtr(now.Add(-24 * time.Hour)), hotScore: 78.1,
			likes: 18, favorites: 5, comments: 4,
			imageURLs: []string{"https://picsum.photos/seed/react19/800/600", "https://picsum.photos/seed/react-server/800/600"},
			tagIDs:    []uint64{tagReact.ID, tagTypescript.ID, tagTutorial.ID},
		},
		{
			title: "TailwindCSS v4 迁移指南：从配置文件到零配置",
			body: "TailwindCSS v4 带来了颠覆性的变化：不再需要 tailwind.config.js。\n\n## 核心变化\n\n1. CSS-first 配置：所有配置都在 CSS 文件中通过 @theme 指令完成\n2. 自动检测：不再需要配置 content 路径\n3. 更快的编译：Rust 引擎重写，编译速度提升 10 倍\n\n## 迁移步骤\n\n1. 升级依赖\n2. 删除配置文件\n3. 将自定义主题迁移到 CSS\n\n整个过程大概 30 分钟就能完成。推荐大家尽早迁移。",
			authorID: user4.ID, status: note.StatusPublished,
			publishedAt: timePtr(now.Add(-12 * time.Hour)), hotScore: 65.4,
			likes: 12, favorites: 7, comments: 3,
			imageURLs: []string{"https://picsum.photos/seed/tailwind/800/600"},
			tagIDs:    []uint64{tagReact.ID, tagDesign.ID},
		},
		{
			title: "Redis 缓存实战：如何避免缓存击穿、穿透、雪崩",
			body: "缓存是高并发系统的基石，但使用不当会引入更多问题。\n\n## 三大缓存问题\n\n### 缓存穿透\n查询不存在的数据，请求直接打到数据库。解决方案：布隆过滤器、缓存空值。\n\n### 缓存击穿\n热点 key 过期瞬间，大量请求涌入数据库。解决方案：互斥锁、逻辑过期。\n\n### 缓存雪崩\n大量 key 同时过期。解决方案：过期时间加随机偏移、多级缓存。\n\n这套方案在压测中表现稳定。",
			authorID: user2.ID, status: note.StatusPublished,
			publishedAt: timePtr(now.Add(-6 * time.Hour)), hotScore: 71.8,
			likes: 20, favorites: 10, comments: 5,
			imageURLs: []string{"https://picsum.photos/seed/redis-cache/800/600", "https://picsum.photos/seed/redis-arch/800/600"},
			tagIDs:    []uint64{tagBackend.ID, tagRedis.ID, tagTutorial.ID},
		},
		{
			title: "Docker Compose 多服务联调技巧",
			body: "在微服务开发中，Docker Compose 是联调的利器。本文分享几个实用技巧。\n\n## 注意事项\n\n1. 服务间用服务名而不是 localhost\n2. 健康检查比 depends_on 更可靠\n3. 开发时挂载代码目录实现热重载\n4. 用 profiles 区分开发和生产配置\n\n这些技巧能帮你节省大量联调时间。",
			authorID: user3.ID, status: note.StatusPublished,
			publishedAt: timePtr(now.Add(-3 * time.Hour)), hotScore: 55.2,
			likes: 9, favorites: 4, comments: 2,
			tagIDs: []uint64{tagDocker.ID, tagMicroservice.ID},
		},
		{
			title: "TypeScript 类型体操入门：从基础到进阶",
			body: "TypeScript 的类型系统是图灵完备的，可以做很多有趣的事情。\n\n## 基础工具类型\n\nPartial 让所有属性变为可选\nPick 从 T 中选取部分属性\nOmit 从 T 中排除部分属性\nRecord 构造键值对类型\n\n## 进阶：条件类型\n\n条件类型结合 infer 关键字可以实现类型推断，比如从函数类型中提取返回值类型。\n\n掌握类型体操可以让你写出更安全的代码。但记住：可读性大于技巧性。",
			authorID: user3.ID, status: note.StatusPublished,
			publishedAt: timePtr(now.Add(-1 * time.Hour)), hotScore: 48.7,
			likes: 15, favorites: 6, comments: 3,
			imageURLs: []string{"https://picsum.photos/seed/typescript/800/600"},
			tagIDs:    []uint64{tagTypescript.ID, tagTutorial.ID},
		},
		{
			title: "程序员如何做好技术分享（草稿）",
			body:    "技术分享是提升个人影响力的重要方式。这篇文章还在写，先保存草稿。",
			authorID: user4.ID, status: note.StatusPendingReview,
			tagIDs: []uint64{tagCareer.ID},
		},
	}

	var created []*note.Note
	for _, n := range notes {
		nn := &note.Note{
			AuthorID: n.authorID, Title: n.title, Body: n.body,
			Status: n.status, Visibility: note.VisibilityPublic,
			PublishedAt: n.publishedAt, HotScore: n.hotScore,
			LikesCount: n.likes, FavoritesCount: n.favorites, CommentsCount: n.comments,
			CreatedAt: now, UpdatedAt: now,
		}
		if err := db.Create(nn).Error; err != nil {
			log.Fatalf("create note %q: %v", n.title, err)
		}
		created = append(created, nn)
		for i, url := range n.imageURLs {
			db.Create(&note.NoteImage{NoteID: nn.ID, URL: url, StorageKey: fmt.Sprintf("seed_%d", nn.ID), SortOrder: i})
		}
		for _, tid := range n.tagIDs {
			db.Create(&note.NoteTag{NoteID: nn.ID, TagID: tid})
			db.Model(&note.Tag{}).Where("id = ?", tid).UpdateColumn("notes_count", gorm.Expr("notes_count + 1"))
		}
		if n.status == note.StatusPublished && n.publishedAt != nil {
			d, rl, r, tr := "pass", "low", "Auto-approved by seed", fmt.Sprintf("seed_trace_%d", nn.ID)
			db.Create(&review.ReviewTask{
				NoteID: nn.ID, AuthorID: n.authorID, Status: review.TaskStatusAgentPassed, Source: "publish",
				AgentDecision: &d, AgentRiskLevel: &rl, AgentReason: &r, AgentTraceID: &tr,
				DecidedAt: n.publishedAt, CreatedAt: *n.publishedAt, UpdatedAt: *n.publishedAt,
			})
		} else if n.status == note.StatusPendingReview {
			db.Create(&review.ReviewTask{NoteID: nn.ID, AuthorID: n.authorID, Status: review.TaskStatusPendingAgent, Source: "publish", CreatedAt: now, UpdatedAt: now})
		}
	}

	log.Println("Creating interactions...")
	if len(created) >= 7 {
		batchLikes(db, user3.ID, created[0], created[1], created[4])
		batchLikes(db, user4.ID, created[0], created[2], created[3])
		batchLikes(db, user2.ID, created[2], created[3])
		batchFavs(db, user3.ID, created[1], created[4])
		batchFavs(db, user4.ID, created[0], created[2])
		addComment(db, user3.ID, created[0].ID, "泛型确实好用，我项目里也用泛型工具函数大幅减少了代码量！")
		addComment(db, user4.ID, created[0].ID, "求分享完整的 Repository 泛型封装～")
		addComment(db, user2.ID, created[2].ID, "Server Components 首屏性能提升太明显了，FCP 降了 3 倍。")
		addComment(db, user2.ID, created[1].ID, "Outbox 模式这个选择很关键，我们之前用过 AfterCommit hook，数据一致性经常出问题。")
		addComment(db, user4.ID, created[4].ID, "加随机偏移这个技巧很实用，之前雪崩过一次，血的教训。")
		addComment(db, user3.ID, created[4].ID, "想问下布隆过滤器你们用的什么实现？Go 里有推荐的库吗？")
	}

	log.Println("Creating follows...")
	db.Create(&interaction.Follow{FollowerID: user3.ID, FolloweeID: user2.ID, CreatedAt: now.Add(-48 * time.Hour)})
	db.Create(&interaction.Follow{FollowerID: user4.ID, FolloweeID: user2.ID, CreatedAt: now.Add(-36 * time.Hour)})
	db.Create(&interaction.Follow{FollowerID: user2.ID, FolloweeID: user3.ID, CreatedAt: now.Add(-24 * time.Hour)})
	db.Create(&interaction.Follow{FollowerID: user4.ID, FolloweeID: user3.ID, CreatedAt: now.Add(-12 * time.Hour)})
	db.Create(&interaction.Follow{FollowerID: user2.ID, FolloweeID: user4.ID, CreatedAt: now.Add(-6 * time.Hour)})

	fmt.Println("\n========================================")
	fmt.Println("  Seed complete!")
	fmt.Println("========================================")
	fmt.Println("\nTest Accounts:")
	fmt.Println("  Admin:    admin@ripple.dev  / admin123")
	fmt.Println("  User:     alice@ripple.dev  / alice123")
	fmt.Println("  User:     bob@ripple.dev    / bob123")
	fmt.Println("  User:     carol@ripple.dev  / carol123")
	fmt.Printf("\nCreated: %d users, %d notes, %d tags, comments, likes, follows\n", 4, len(created), 10)
}

func createUser(db *gorm.DB, hasher auth.PasswordHasher, email, password, nickname, role, bio string) *account.User {
	var u account.User
	if db.Where("email = ?", email).First(&u).Error == nil {
		return &u
	}
	hash, _ := hasher.Hash(password)
	now := time.Now()
	u = account.User{
		Email: email, PasswordHash: hash, Nickname: nickname,
		AvatarURL: fmt.Sprintf("https://i.pravatar.cc/150?u=%s", email),
		Bio: bio, Role: role, Status: account.StatusActive,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&u).Error; err != nil {
		log.Fatalf("create user %s: %v", email, err)
	}
	return &u
}

func createTag(db *gorm.DB, name string) note.Tag {
	var t note.Tag
	if db.Where("name = ?", name).First(&t).Error == nil {
		return t
	}
	now := time.Now()
	t = note.Tag{Name: name, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&t).Error; err != nil {
		log.Fatalf("create tag %s: %v", name, err)
	}
	return t
}

func batchLikes(db *gorm.DB, userID uint64, notes ...*note.Note) {
	for _, n := range notes {
		db.Create(&interaction.NoteLike{UserID: userID, NoteID: n.ID, CreatedAt: time.Now()})
	}
}

func batchFavs(db *gorm.DB, userID uint64, notes ...*note.Note) {
	for _, n := range notes {
		db.Create(&interaction.NoteFavorite{UserID: userID, NoteID: n.ID, CreatedAt: time.Now()})
	}
}

func addComment(db *gorm.DB, authorID, noteID uint64, body string) {
	db.Create(&interaction.Comment{NoteID: noteID, AuthorID: authorID, Body: body, Status: interaction.CommentStatusVisible})
}

func timePtr(t time.Time) *time.Time { return &t }
