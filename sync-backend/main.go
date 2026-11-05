package main

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"uuid"
)

// Command 命令接口
type Command interface {
	Name() string
	Description() string
	Run(args []string) error
}

// ServerCommand 启动服务器命令
type ServerCommand struct{}

func (c *ServerCommand) Name() string        { return "server" }
func (c *ServerCommand) Description() string { return "Start the sync server" }

func (c *ServerCommand) Run(args []string) error {
	fs := flag.NewFlagSet(c.Name(), flag.ExitOnError)
	configPath := fs.String("config", "config.toml", "Path to config file")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	// 初始化应用
	app, err := c.initializeApp(*configPath)
	if err != nil {
		return err
	}
	defer app.Close()

	// 启动服务器
	if err := app.Start(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	fmt.Println("Server started successfully")

	// 等待中断信号
	c.waitForShutdown(app)

	fmt.Println("Server stopped")
	return nil
}

func (c *ServerCommand) initializeApp(configPath string) (*Application, error) {
	// 加载配置
	config, err := LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// 连接数据库
	db, err := NewDB(config.Data.Database)
	if err != nil {
		return nil, fmt.Errorf("failed to connect database: %w", err)
	}

	// 执行迁移
	if err := db.Migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	// 创建存储
	storage, err := NewStorage(config.Data.DataDir)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create storage: %w", err)
	}

	return NewApplication(config, db, storage), nil
}

func (c *ServerCommand) waitForShutdown(app *Application) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nShutting down server...")
	if err := app.Stop(); err != nil {
		fmt.Printf("Error stopping server: %v\n", err)
	}
}

// GenUserCommand 生成用户命令
type GenUserCommand struct{}

func (c *GenUserCommand) Name() string        { return "gen-user" }
func (c *GenUserCommand) Description() string { return "Generate a new user sync ID and key" }

func (c *GenUserCommand) Run(args []string) error {
	fs := flag.NewFlagSet(c.Name(), flag.ExitOnError)
	configPath := fs.String("config", "config.toml", "Path to config file")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	// 初始化服务
	service, err := c.initializeService(*configPath)
	if err != nil {
		return err
	}
	defer service.Close()

	// 生成用户凭证
	syncID := generateSyncID()
	userKey := generateUserKey()

	// 创建用户
	if _, err := service.CreateUser(syncID, userKey); err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	c.printUserCredentials(syncID, userKey)
	return nil
}

func (c *GenUserCommand) initializeService(configPath string) (*SyncService, error) {
	config, err := LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	db, err := NewDB(config.Data.Database)
	if err != nil {
		return nil, fmt.Errorf("failed to connect database: %w", err)
	}

	if err := db.Migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	storage, err := NewStorage(config.Data.DataDir)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create storage: %w", err)
	}

	return NewSyncService(db, storage, config.Data.MaxVersions), nil
}

func (c *GenUserCommand) printUserCredentials(syncID, userKey string) {
	fmt.Println("User created successfully!")
	fmt.Println()
	fmt.Printf("Sync ID:  %s\n", syncID)
	fmt.Printf("User Key: %s\n", userKey)
	fmt.Println()
	fmt.Println("Please save these credentials securely.")
	fmt.Println("The User Key is used only for API authentication.")
}

// Application 应用程序
type Application struct {
	config      *ServerConfig
	db          *DB
	storage     *Storage
	server      *HTTPServer
	handler     *Handler
	service     *SyncService
	rateLimiter *RateLimiter
}

// NewApplication 创建应用程序
func NewApplication(config *ServerConfig, db *DB, storage *Storage) *Application {
	service := NewSyncService(db, storage, config.Data.MaxVersions)
	rateLimiter := DefaultRateLimiter(service)
	handler := NewHandler(service, rateLimiter)
	server := NewHTTPServer(config, handler.Routes())

	return &Application{
		config:      config,
		db:          db,
		storage:     storage,
		server:      server,
		handler:     handler,
		service:     service,
		rateLimiter: rateLimiter,
	}
}

// Start 启动应用
func (app *Application) Start() error {
	go func() {
		if err := app.server.Start(); err != nil {
			fmt.Printf("Server error: %v\n", err)
			os.Exit(1)
		}
	}()
	return nil
}

// Stop 停止应用
func (app *Application) Stop() error {
	return app.server.Stop()
}

// Close 关闭应用资源
func (app *Application) Close() {
	if app.db != nil {
		app.db.Close()
	}
}

// CommandRegistry 命令注册表
type CommandRegistry struct {
	commands map[string]Command
}

// NewCommandRegistry 创建命令注册表
func NewCommandRegistry() *CommandRegistry {
	return &CommandRegistry{
		commands: make(map[string]Command),
	}
}

// Register 注册命令
func (r *CommandRegistry) Register(cmd Command) {
	r.commands[cmd.Name()] = cmd
}

// Get 获取命令
func (r *CommandRegistry) Get(name string) (Command, bool) {
	cmd, ok := r.commands[name]
	return cmd, ok
}

// Names 获取所有命令名称
func (r *CommandRegistry) Names() []string {
	names := make([]string, 0, len(r.commands))
	for name := range r.commands {
		names = append(names, name)
	}
	return names
}

// PrintUsage 打印使用帮助
func (r *CommandRegistry) PrintUsage() {
	fmt.Println("Usage: sync-backend <command> [options]")
	fmt.Println()
	fmt.Println("Commands:")
	for _, name := range r.Names() {
		if cmd, ok := r.Get(name); ok {
			fmt.Printf("  %-12s %s\n", cmd.Name(), cmd.Description())
		}
	}
	fmt.Println()
	fmt.Println("Use 'sync-backend <command> -h' for command-specific help")
}

func main() {
	registry := NewCommandRegistry()
	registry.Register(&ServerCommand{})
	registry.Register(&GenUserCommand{})

	if len(os.Args) < 2 {
		registry.PrintUsage()
		os.Exit(1)
	}

	cmdName := os.Args[1]
	cmd, ok := registry.Get(cmdName)
	if !ok {
		fmt.Printf("Unknown command: %s\n", cmdName)
		registry.PrintUsage()
		os.Exit(1)
	}

	if err := cmd.Run(os.Args[2:]); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

// generateSyncID 生成同步ID
func generateSyncID() string {
	return uuid.New().String()
}

// generateUserKey 生成用户密钥
func generateUserKey() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}
