package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	_ "github.com/go-sql-driver/mysql"
)

const (
	defaultPort            = "8080"
	defaultDBPort          = "3306"
	defaultDBType          = "mysql"
	defaultMaxOpenConns    = 25
	defaultMaxIdleConns    = 10
	defaultConnMaxLifetime = 5 * time.Minute
	defaultConnMaxIdleTime = 2 * time.Minute
	requestTimeout         = 3 * time.Second
	startupRetryCount      = 24
	startupRetryDelay      = 5 * time.Second
	dynamoCounterKey       = "__counter__"
)

const createCartsTableSQL = `
CREATE TABLE IF NOT EXISTS carts (
    cart_id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    customer_id BIGINT UNSIGNED NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (cart_id),
    KEY idx_carts_customer_id (customer_id),
    KEY idx_carts_customer_updated (customer_id, updated_at DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
`

const createCartItemsTableSQL = `
CREATE TABLE IF NOT EXISTS cart_items (
    cart_item_id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    cart_id BIGINT UNSIGNED NOT NULL,
    product_id BIGINT UNSIGNED NOT NULL,
    quantity INT UNSIGNED NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (cart_item_id),
    CONSTRAINT chk_cart_items_quantity CHECK (quantity > 0),
    CONSTRAINT uq_cart_items_cart_product UNIQUE (cart_id, product_id),
    CONSTRAINT fk_cart_items_cart
        FOREIGN KEY (cart_id)
        REFERENCES carts (cart_id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
`

type configValues struct {
	port            string
	dbType          string
	dbHost          string
	dbPort          string
	dbName          string
	dbUser          string
	dbPassword      string
	maxOpenConns    int
	maxIdleConns    int
	connMaxLifetime time.Duration
	connMaxIdleTime time.Duration
	awsRegion       string
	dynamoTableName string
}

type app struct {
	store  CartStore
	dbType string
	dbHost string
}

type CartStore interface {
	HealthCheck(context.Context) error
	CreateCart(context.Context, int64) (*cartResponse, error)
	GetCart(context.Context, int64) (*cartResponse, error)
	AddItem(context.Context, int64, int64, int64) error
	Close() error
}

type mysqlCartStore struct {
	db *sql.DB
}

type dynamoCartStore struct {
	client    *dynamodb.Client
	tableName string
}

type dynamoCart struct {
	CartID     string     `dynamodbav:"cart_id"`
	CustomerID int64      `dynamodbav:"customer_id"`
	Status     string     `dynamodbav:"status"`
	Items      []cartItem `dynamodbav:"items"`
	CreatedAt  string     `dynamodbav:"created_at"`
	UpdatedAt  string     `dynamodbav:"updated_at"`
}

type createCartRequest struct {
	CustomerID int64 `json:"customer_id"`
}

type addItemRequest struct {
	ProductID int64 `json:"product_id"`
	Quantity  int64 `json:"quantity"`
}

type errorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

type cartItem struct {
	ProductID int64 `json:"product_id" dynamodbav:"product_id"`
	Quantity  int64 `json:"quantity" dynamodbav:"quantity"`
}

type cartResponse struct {
	ShoppingCartID int64      `json:"shopping_cart_id"`
	CustomerID     int64      `json:"customer_id"`
	Status         string     `json:"status"`
	Items          []cartItem `json:"items"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type healthResponse struct {
	Status  string `json:"status"`
	Backend string `json:"backend"`
	DBHost  string `json:"db_host,omitempty"`
	Table   string `json:"table,omitempty"`
}

var errCartNotFound = errors.New("shopping cart not found")

func main() {
	cfg := mustLoadConfig()

	store, err := openStore(cfg)
	if err != nil {
		log.Fatalf("store initialization failed: %v", err)
	}
	defer func() {
		_ = store.Close()
	}()

	server := &http.Server{
		Addr:              ":" + cfg.port,
		Handler:           (&app{store: store, dbType: cfg.dbType, dbHost: cfg.dbHost}).routes(cfg.dynamoTableName),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("listening on :%s using backend=%s", cfg.port, cfg.dbType)
	log.Fatal(server.ListenAndServe())
}

func mustLoadConfig() configValues {
	cfg := configValues{
		port:            envOrDefault("PORT", defaultPort),
		dbType:          strings.ToLower(envOrDefault("DB_TYPE", defaultDBType)),
		dbHost:          strings.TrimSpace(os.Getenv("DB_HOST")),
		dbPort:          envOrDefault("DB_PORT", defaultDBPort),
		dbName:          strings.TrimSpace(os.Getenv("DB_NAME")),
		dbUser:          strings.TrimSpace(os.Getenv("DB_USER")),
		dbPassword:      os.Getenv("DB_PASSWORD"),
		maxOpenConns:    envIntOrDefault("DB_MAX_OPEN_CONNS", defaultMaxOpenConns),
		maxIdleConns:    envIntOrDefault("DB_MAX_IDLE_CONNS", defaultMaxIdleConns),
		connMaxLifetime: envDurationOrDefault("DB_CONN_MAX_LIFETIME", defaultConnMaxLifetime),
		connMaxIdleTime: envDurationOrDefault("DB_CONN_MAX_IDLE_TIME", defaultConnMaxIdleTime),
		awsRegion:       strings.TrimSpace(os.Getenv("AWS_REGION")),
		dynamoTableName: strings.TrimSpace(os.Getenv("DYNAMODB_TABLE_NAME")),
	}

	switch cfg.dbType {
	case "mysql":
		missing := make([]string, 0, 4)
		if cfg.dbHost == "" {
			missing = append(missing, "DB_HOST")
		}
		if cfg.dbName == "" {
			missing = append(missing, "DB_NAME")
		}
		if cfg.dbUser == "" {
			missing = append(missing, "DB_USER")
		}
		if cfg.dbPassword == "" {
			missing = append(missing, "DB_PASSWORD")
		}
		if len(missing) > 0 {
			log.Fatalf("missing required environment variables for mysql: %s", strings.Join(missing, ", "))
		}
	case "dynamodb":
		missing := make([]string, 0, 2)
		if cfg.awsRegion == "" {
			missing = append(missing, "AWS_REGION")
		}
		if cfg.dynamoTableName == "" {
			missing = append(missing, "DYNAMODB_TABLE_NAME")
		}
		if len(missing) > 0 {
			log.Fatalf("missing required environment variables for dynamodb: %s", strings.Join(missing, ", "))
		}
	default:
		log.Fatalf("unsupported DB_TYPE %q", cfg.dbType)
	}

	return cfg
}

func openStore(cfg configValues) (CartStore, error) {
	switch cfg.dbType {
	case "mysql":
		return openMySQLStore(cfg)
	case "dynamodb":
		return openDynamoStore(cfg)
	default:
		return nil, fmt.Errorf("unsupported DB_TYPE %q", cfg.dbType)
	}
}

func openMySQLStore(cfg configValues) (CartStore, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&collation=utf8mb4_unicode_ci&parseTime=true",
		cfg.dbUser,
		cfg.dbPassword,
		cfg.dbHost,
		cfg.dbPort,
		cfg.dbName,
	)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(cfg.maxOpenConns)
	db.SetMaxIdleConns(cfg.maxIdleConns)
	db.SetConnMaxLifetime(cfg.connMaxLifetime)
	db.SetConnMaxIdleTime(cfg.connMaxIdleTime)

	if err := waitForDatabase(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := runMigrations(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	return &mysqlCartStore{db: db}, nil
}

func openDynamoStore(cfg configValues) (CartStore, error) {
	awsCfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(cfg.awsRegion))
	if err != nil {
		return nil, err
	}

	store := &dynamoCartStore{
		client:    dynamodb.NewFromConfig(awsCfg),
		tableName: cfg.dynamoTableName,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := store.HealthCheck(ctx); err != nil {
		return nil, err
	}

	return store, nil
}

func waitForDatabase(db *sql.DB) error {
	var lastErr error
	for attempt := 1; attempt <= startupRetryCount; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		lastErr = db.PingContext(ctx)
		cancel()
		if lastErr == nil {
			return nil
		}

		log.Printf("database not ready yet (attempt %d/%d): %v", attempt, startupRetryCount, lastErr)
		time.Sleep(startupRetryDelay)
	}

	return fmt.Errorf("database ping failed after retries: %w", lastErr)
}

func runMigrations(db *sql.DB) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := db.ExecContext(ctx, createCartsTableSQL); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, createCartItemsTableSQL); err != nil {
		return err
	}
	return nil
}

func (a *app) routes(dynamoTable string) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			notFound(w)
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{
			"service": "hw8-cart-api",
			"status":  "ok",
		})
	})

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
		defer cancel()

		status := http.StatusOK
		payload := healthResponse{Status: "ok", Backend: a.dbType}
		if a.dbType == "mysql" {
			payload.DBHost = a.dbHost
		}
		if a.dbType == "dynamodb" {
			payload.Table = dynamoTable
		}
		if err := a.store.HealthCheck(ctx); err != nil {
			status = http.StatusServiceUnavailable
			payload.Status = "degraded"
		}

		writeJSON(w, status, payload)
	})

	mux.HandleFunc("/shopping-carts", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/shopping-carts" {
			notFound(w)
			return
		}
		if r.Method != http.MethodPost {
			methodNotAllowed(w, http.MethodPost)
			return
		}

		a.handleCreateCart(w, r)
	})

	mux.HandleFunc("/shopping-carts/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/shopping-carts/")
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) == 0 || parts[0] == "" {
			notFound(w)
			return
		}

		cartID, ok := parsePositiveInt64(parts[0])
		if !ok {
			writeError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid shopping cart ID", "shoppingCartId must be a positive integer")
			return
		}

		if len(parts) == 1 {
			if r.Method != http.MethodGet {
				methodNotAllowed(w, http.MethodGet)
				return
			}

			a.handleGetCart(w, r, cartID)
			return
		}

		if len(parts) == 2 && parts[1] == "items" {
			if r.Method != http.MethodPost {
				methodNotAllowed(w, http.MethodPost)
				return
			}

			a.handleAddItem(w, r, cartID)
			return
		}

		notFound(w)
	})

	return mux
}

func (a *app) handleCreateCart(w http.ResponseWriter, r *http.Request) {
	var req createCartRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid input data", err.Error())
		return
	}
	if req.CustomerID < 1 {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid input data", "customer_id must be >= 1")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	cart, err := a.store.CreateCart(ctx, req.CustomerID)
	if err != nil {
		log.Printf("create cart failed: %v", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error", "Could not create shopping cart")
		return
	}

	writeJSON(w, http.StatusCreated, cart)
}

func (a *app) handleGetCart(w http.ResponseWriter, r *http.Request, cartID int64) {
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	cart, err := a.store.GetCart(ctx, cartID)
	if err != nil {
		if errors.Is(err, errCartNotFound) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Shopping cart not found", "No shopping cart with that ID")
			return
		}

		log.Printf("get cart failed: %v", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error", "Could not retrieve shopping cart")
		return
	}

	writeJSON(w, http.StatusOK, cart)
}

func (a *app) handleAddItem(w http.ResponseWriter, r *http.Request, cartID int64) {
	var req addItemRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid input data", err.Error())
		return
	}
	if req.ProductID < 1 {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid input data", "product_id must be >= 1")
		return
	}
	if req.Quantity < 1 {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid input data", "quantity must be >= 1")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	err := a.store.AddItem(ctx, cartID, req.ProductID, req.Quantity)
	if err != nil {
		if errors.Is(err, errCartNotFound) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Shopping cart not found", "No shopping cart with that ID")
			return
		}

		log.Printf("add item failed: %v", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error", "Could not update shopping cart items")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *mysqlCartStore) HealthCheck(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *mysqlCartStore) CreateCart(ctx context.Context, customerID int64) (*cartResponse, error) {
	result, err := s.db.ExecContext(ctx, `
        INSERT INTO carts (customer_id)
        VALUES (?)
    `, customerID)
	if err != nil {
		return nil, err
	}

	cartID, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	cart := &cartResponse{}
	err = s.db.QueryRowContext(ctx, `
        SELECT cart_id, customer_id, status, created_at, updated_at
        FROM carts
        WHERE cart_id = ?
    `, cartID).Scan(
		&cart.ShoppingCartID,
		&cart.CustomerID,
		&cart.Status,
		&cart.CreatedAt,
		&cart.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	cart.Items = []cartItem{}
	return cart, nil
}

func (s *mysqlCartStore) GetCart(ctx context.Context, cartID int64) (*cartResponse, error) {
	rows, err := s.db.QueryContext(ctx, `
        SELECT
            c.cart_id,
            c.customer_id,
            c.status,
            c.created_at,
            c.updated_at,
            ci.product_id,
            ci.quantity
        FROM carts c
        LEFT JOIN cart_items ci ON ci.cart_id = c.cart_id
        WHERE c.cart_id = ?
        ORDER BY ci.cart_item_id
    `, cartID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cart *cartResponse
	for rows.Next() {
		var (
			rowCart   cartResponse
			productID sql.NullInt64
			quantity  sql.NullInt64
		)

		if err := rows.Scan(
			&rowCart.ShoppingCartID,
			&rowCart.CustomerID,
			&rowCart.Status,
			&rowCart.CreatedAt,
			&rowCart.UpdatedAt,
			&productID,
			&quantity,
		); err != nil {
			return nil, err
		}

		if cart == nil {
			rowCart.Items = []cartItem{}
			cart = &rowCart
		}

		if productID.Valid && quantity.Valid {
			cart.Items = append(cart.Items, cartItem{
				ProductID: productID.Int64,
				Quantity:  quantity.Int64,
			})
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	if cart == nil {
		return nil, errCartNotFound
	}

	return cart, nil
}

func (s *mysqlCartStore) AddItem(ctx context.Context, cartID, productID, quantity int64) (err error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	result, err := tx.ExecContext(ctx, `
        UPDATE carts
        SET updated_at = CURRENT_TIMESTAMP
        WHERE cart_id = ?
    `, cartID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return errCartNotFound
	}

	_, err = tx.ExecContext(ctx, `
        INSERT INTO cart_items (cart_id, product_id, quantity)
        VALUES (?, ?, ?)
        ON DUPLICATE KEY UPDATE
            quantity = quantity + VALUES(quantity),
            updated_at = CURRENT_TIMESTAMP
    `, cartID, productID, quantity)
	if err != nil {
		return err
	}

	err = tx.Commit()
	return err
}

func (s *mysqlCartStore) Close() error {
	return s.db.Close()
}

func (s *dynamoCartStore) HealthCheck(ctx context.Context) error {
	_, err := s.client.DescribeTable(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(s.tableName),
	})
	return err
}

func (s *dynamoCartStore) CreateCart(ctx context.Context, customerID int64) (*cartResponse, error) {
	cartID, err := s.nextCartID(ctx)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	item := dynamoCart{
		CartID:     strconv.FormatInt(cartID, 10),
		CustomerID: customerID,
		Status:     "active",
		Items:      []cartItem{},
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return nil, err
	}

	_, err = s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.tableName),
		Item:      av,
	})
	if err != nil {
		return nil, err
	}

	return dynamoToCartResponse(item)
}

func (s *dynamoCartStore) GetCart(ctx context.Context, cartID int64) (*cartResponse, error) {
	output, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"cart_id": &types.AttributeValueMemberS{Value: strconv.FormatInt(cartID, 10)},
		},
	})
	if err != nil {
		return nil, err
	}
	if len(output.Item) == 0 {
		return nil, errCartNotFound
	}

	var item dynamoCart
	if err := attributevalue.UnmarshalMap(output.Item, &item); err != nil {
		return nil, err
	}

	return dynamoToCartResponse(item)
}

func (s *dynamoCartStore) AddItem(ctx context.Context, cartID, productID, quantity int64) error {
	cart, err := s.GetCart(ctx, cartID)
	if err != nil {
		return err
	}

	items := cart.Items
	found := false
	for i := range items {
		if items[i].ProductID == productID {
			items[i].Quantity += quantity
			found = true
			break
		}
	}
	if !found {
		items = append(items, cartItem{
			ProductID: productID,
			Quantity:  quantity,
		})
	}

	itemsAV, err := attributevalue.Marshal(items)
	if err != nil {
		return err
	}

	updatedAt := time.Now().UTC().Format(time.RFC3339)
	_, err = s.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"cart_id": &types.AttributeValueMemberS{Value: strconv.FormatInt(cartID, 10)},
		},
		UpdateExpression:    aws.String("SET #items = :items, updated_at = :updated_at"),
		ConditionExpression: aws.String("attribute_exists(cart_id)"),
		ExpressionAttributeNames: map[string]string{
			"#items": "items",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":items":      itemsAV,
			":updated_at": &types.AttributeValueMemberS{Value: updatedAt},
		},
	})
	var conditionalErr *types.ConditionalCheckFailedException
	if errors.As(err, &conditionalErr) {
		return errCartNotFound
	}
	return err
}

func (s *dynamoCartStore) Close() error {
	return nil
}

func (s *dynamoCartStore) nextCartID(ctx context.Context) (int64, error) {
	output, err := s.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"cart_id": &types.AttributeValueMemberS{Value: dynamoCounterKey},
		},
		UpdateExpression: aws.String("ADD counter_value :inc"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":inc": &types.AttributeValueMemberN{Value: "1"},
		},
		ReturnValues: types.ReturnValueUpdatedNew,
	})
	if err != nil {
		return 0, err
	}

	attr, ok := output.Attributes["counter_value"]
	if !ok {
		return 0, errors.New("counter_value missing from dynamodb counter response")
	}

	var counter int64
	if err := attributevalue.Unmarshal(attr, &counter); err != nil {
		return 0, err
	}
	return counter, nil
}

func dynamoToCartResponse(item dynamoCart) (*cartResponse, error) {
	cartID, err := strconv.ParseInt(item.CartID, 10, 64)
	if err != nil {
		return nil, err
	}
	createdAt, err := time.Parse(time.RFC3339, item.CreatedAt)
	if err != nil {
		return nil, err
	}
	updatedAt, err := time.Parse(time.RFC3339, item.UpdatedAt)
	if err != nil {
		return nil, err
	}

	items := item.Items
	if items == nil {
		items = []cartItem{}
	}

	return &cartResponse{
		ShoppingCartID: cartID,
		CustomerID:     item.CustomerID,
		Status:         item.Status,
		Items:          items,
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
	}, nil
}

func decodeJSON(r *http.Request, dst any) error {
	if r.Body == nil {
		return errors.New("request body is required")
	}
	defer r.Body.Close()

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		return err
	}
	if dec.More() {
		return errors.New("request body must contain a single JSON object")
	}

	return nil
}

func parsePositiveInt64(raw string) (int64, bool) {
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value < 1 {
		return 0, false
	}
	return value, true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message, details string) {
	writeJSON(w, status, errorResponse{
		Error:   code,
		Message: message,
		Details: details,
	})
}

func methodNotAllowed(w http.ResponseWriter, allowedMethod string) {
	w.Header().Set("Allow", allowedMethod)
	writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed", "Use "+allowedMethod)
}

func notFound(w http.ResponseWriter) {
	writeError(w, http.StatusNotFound, "NOT_FOUND", "Not found", "Resource does not exist")
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envIntOrDefault(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}

	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		log.Fatalf("invalid integer value for %s: %q", key, raw)
	}
	return value
}

func envDurationOrDefault(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}

	value, err := time.ParseDuration(raw)
	if err != nil {
		log.Fatalf("invalid duration value for %s: %q", key, raw)
	}
	return value
}
