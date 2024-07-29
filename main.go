package main

import (
	"context"
	"html/template"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/go-pg/pg/v10"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"golang.org/x/exp/rand"
)

var tmpl *template.Template

type User struct {
	ID      uuid.UUID `json:"id" pg:"id,type:uuid,default:uuid_generate_v4()"`
	Name    string    `json:"name" pg:"name"`
	Address string    `json:"address" pg:"address"`
	Orders  []Order   `json:"orders" pg:"rel:has-many"`
	Reviews []Review  `json:"reviews" pg:"rel:has-many"`
}

type Product struct {
	ID          uuid.UUID `json:"id" pg:"id,type:uuid,default:uuid_generate_v4()"`
	Name        string    `json:"name" pg:"name"`
	Description string    `json:"description" pg:"description"`
	Price       float64   `json:"price" pg:"price"`
}

type Order struct {
	ID         uuid.UUID `json:"id" pg:"id,type:uuid,default:uuid_generate_v4()"`
	UserID     uuid.UUID `json:"user_id" pg:"user_id,type:uuid"`
	User       *User     `json:"user" pg:"rel:has-one"`
	ProductID  uuid.UUID `json:"product_id" pg:"product_id,type:uuid"`
	Product    *Product  `json:"product" pg:"rel:has-one"`
	Quantity   int       `json:"quantity" pg:"quantity"`
	TotalPrice float64   `json:"total_price" pg:"total_price"`
	Date       time.Time `json:"date" pg:"date"`
}

type Review struct {
	ID        uuid.UUID `json:"id" pg:"id,type:uuid,default:uuid_generate_v4()"`
	ProductID uuid.UUID `json:"product_id" pg:"product_id,type:uuid"`
	Product   *Product  `json:"product" pg:"rel:has-one"`
	UserID    uuid.UUID `json:"user_id" pg:"user_id,type:uuid"`
	User      *User     `json:"user" pg:"rel:has-one"`
	Rating    int       `json:"rating" pg:"rating"`
	Content   string    `json:"content" pg:"content"`
}

type App struct {
	db *pg.DB
}

// mustEnv retrieves an environment variable with the given key, panicking if it's not set
func mustEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		panic("missing required environment variable: " + key)
	}
	return val
}

func pgOptionsFromEnv() *pg.Options {
	addr := mustEnv("POSTGRES_ADDR")
	user := mustEnv("POSTGRES_USER")
	password := mustEnv("POSTGRES_PASSWORD")
	database := mustEnv("POSTGRES_DB")
	return &pg.Options{
		Addr:     addr,
		User:     user,
		Password: password,
		Database: database,
	}
}

func main() {
	opts := pgOptionsFromEnv()
	db := pg.Connect(opts)
	defer db.Close()

	// TODO: if there's an error connecting, will db be nil?
	if db == nil {
		panic("Error connecting to the database")
	}

	// Initialize our "App" which will make our database accessible to all handlers
	app := &App{db: db}

	tmpl = template.Must(template.ParseGlob("templates/*.html"))

	// seeding
	http.HandleFunc("/firstrun", app.handleFirstRun)
	http.HandleFunc("/seed", app.handleSeedDB)

	// testing
	http.HandleFunc("/", app.handleStaticPage)
	http.HandleFunc("/dynamic", app.handleDynamicPage)
	http.HandleFunc("/read", app.handleDBRead)
	http.HandleFunc("/write", app.handleDBWrite)

	log.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// handleStaticPage serves a static HTML page
func (app *App) handleStaticPage(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "static/index.html")
}

// handleDynamicPage serves a dynamic HTML page that shows the current time
func (app *App) handleDynamicPage(w http.ResponseWriter, r *http.Request) {
	data := struct {
		Title string
		Time  string
	}{
		Title: "Dynamic Page",
		Time:  time.Now().Format(time.RFC822),
	}
	tmpl.ExecuteTemplate(w, "dynamic.html", data)
}

// handleDBRead randomly selects a product and retrieves related information before rendering a response
func (app *App) handleDBRead(w http.ResponseWriter, r *http.Request) {
	// I hate that I have to do this -- maybe use the bun library instead? Does that have a cleaner way to get count queries into ints?
	type CountResult struct {
		Count int
	}

	ctx := r.Context()

	// Get a random product
	var product Product
	_, err := app.db.QueryOneContext(ctx, &product, `
		SELECT * FROM products
		ORDER BY RANDOM()
		LIMIT 1
	`)
	if err != nil {
		log.Printf("error selecting random product: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Get number of orders for this product
	var orderCount CountResult
	_, err = app.db.QueryOne(&orderCount, `
		SELECT COUNT(*) FROM orders
		WHERE product_id = ?
	`, product.ID)
	if err != nil {
		log.Printf("error counting orders: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Get number of unique users who ordered this product
	var uniqueUserCount CountResult

	_, err = app.db.QueryOne(&uniqueUserCount, `
    SELECT COUNT(DISTINCT user_id) FROM orders
    WHERE product_id = ?
`, product.ID)
	if err != nil {
		log.Printf("error counting unique users: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Get reviews for this product
	var reviews []struct {
		Username string
		Rating   int
		Content  string
	}
	_, err = app.db.QueryContext(ctx, &reviews, `
		SELECT u.name as username, r.rating, r.content
		FROM reviews r
		JOIN users u ON r.user_id = u.id
		WHERE r.product_id = ?
	`, product.ID)
	if err != nil {
		log.Printf("error fetching reviews: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Compile data to be rendered
	templateData := struct {
		ProductName     string
		OrderCount      int
		UniqueUserCount int
		Reviews         []struct {
			Username string
			Rating   int
			Content  string
		}
	}{
		ProductName:     product.Name,
		OrderCount:      orderCount.Count,
		UniqueUserCount: uniqueUserCount.Count,
		Reviews:         reviews,
	}

	tmpl.ExecuteTemplate(w, "read.html", templateData)
}

// handleDBWrite creates a new order and review for a random product and user
func (app *App) handleDBWrite(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// Get a random product
	var product Product
	_, err := app.db.QueryOneContext(ctx, &product, `
		SELECT * FROM products
		ORDER BY RANDOM()
		LIMIT 1
	`)
	if err != nil {
		log.Printf("error selecting random product: %v", err)
	}

	// Get a random user
	var user User
	_, err = app.db.QueryOneContext(ctx, &user, `
		SELECT * FROM users
		ORDER BY RANDOM()
		LIMIT 1
	`)
	if err != nil {
		log.Printf("error selecting random user: %v", err)
	}

	// Create a new order
	quantity := gofakeit.Number(1, 5)
	order := &Order{
		UserID:     user.ID,
		ProductID:  product.ID,
		Quantity:   quantity,
		TotalPrice: float64(quantity) * product.Price,
		Date:       time.Now(),
	}

	_, err = app.db.ModelContext(ctx, order).Insert()
	if err != nil {
		log.Printf("error inserting new order: %v", err)
	}

	// Create a new review
	review := &Review{
		ProductID: product.ID,
		UserID:    user.ID,
		Rating:    gofakeit.Number(1, 100),
		Content:   gofakeit.Paragraph(1, 3, 10, "."),
	}

	_, err = app.db.ModelContext(ctx, review).Insert()
	if err != nil {
		log.Printf("error inserting new review: %v", err)
	}

	templateData := struct {
		ProductName     string
		UserName        string
		OrderQuantity   int
		OrderTotalPrice float64
		ReviewRating    int
		ReviewContent   string
	}{
		ProductName:     product.Name,
		UserName:        user.Name,
		OrderQuantity:   order.Quantity,
		OrderTotalPrice: order.TotalPrice,
		ReviewRating:    review.Rating,
		ReviewContent:   review.Content,
	}

	tmpl.ExecuteTemplate(w, "write.html", templateData)
}

func (app *App) handleFirstRun(w http.ResponseWriter, r *http.Request) {
	err := setupDatabase(app.db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	// return a 'success' message with a 201.
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Database created successfully"))
}

func (app *App) handleSeedDB(w http.ResponseWriter, r *http.Request) {
	err := seedDatabase(app.db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	// return a 201 with a success message
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte("Database seeded successfully"))
}

func setupDatabase(db *pg.DB) error {
	var err error
	ctx := context.Background()

	log.Printf("setupDatabase: got database %v", db)

	// Enable uuid-ossp extension
	_, err = db.ExecContext(ctx, "CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\";")
	if err != nil {
		log.Printf("error creating uuid-ossp extension: %v", err)
	}

	// Create Users table
	_, err = db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			name TEXT NOT NULL,
			address TEXT NOT NULL
		);
	`)
	if err != nil {
		log.Printf("error creating users table: %v", err)
	}

	// Create Products table
	_, err = db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS products (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			name TEXT NOT NULL,
			description TEXT NOT NULL,
			price NUMERIC(10, 2) NOT NULL
		);
	`)
	if err != nil {
		log.Printf("error creating products table: %v", err)
	}

	// Create Orders table
	_, err = db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS orders (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			user_id UUID NOT NULL REFERENCES users(id),
			product_id UUID NOT NULL REFERENCES products(id),
			quantity INTEGER NOT NULL,
			total_price NUMERIC(10, 2) NOT NULL,
			date TIMESTAMP WITH TIME ZONE NOT NULL
		);
	`)
	if err != nil {
		log.Printf("error creating orders table: %v", err)
	}

	// Create Reviews table
	_, err = db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS reviews (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			product_id UUID NOT NULL REFERENCES products(id),
			user_id UUID NOT NULL REFERENCES users(id),
			rating INTEGER NOT NULL CHECK (rating >= 0 AND rating <= 100),
			content TEXT NOT NULL
		);
	`)
	if err != nil {
		log.Printf("error creating reviews table: %v", err)
	}

	log.Println("Database schema created successfully")
	return nil
}

func seedDatabase(db *pg.DB) error {
	// Seed users
	users := make([]*User, 2000)
	for i := range users {
		users[i] = &User{
			Name:    gofakeit.Name(),
			Address: gofakeit.Address().Street + ", " + gofakeit.Address().City + ", " + gofakeit.Address().Country,
		}
	}
	_, err := db.Model(&users).Insert()
	if err != nil {
		log.Printf("error inserting users: %v", err)
	}
	log.Printf("Inserted %d users", len(users))

	// Seed products
	products := make([]*Product, 100)
	for i := range products {
		products[i] = &Product{
			Name:        gofakeit.ProductName(),
			Description: gofakeit.ProductDescription(),
			Price:       gofakeit.Price(1, 1000),
		}
	}
	_, err = db.Model(&products).Insert()
	if err != nil {
		log.Printf("error inserting products: %v", err)
	}
	log.Printf("Inserted %d products", len(products))

	// Seed orders
	orders := make([]*Order, 30000)
	for i := range orders {
		user := users[rand.Intn(len(users))]
		product := products[rand.Intn(len(products))]
		quantity := gofakeit.Number(1, 10)
		orders[i] = &Order{
			UserID:     user.ID,
			ProductID:  product.ID,
			Quantity:   quantity,
			TotalPrice: float64(quantity) * product.Price,
			Date:       gofakeit.DateRange(time.Now().AddDate(-1, 0, 0), time.Now()),
		}
	}
	_, err = db.Model(&orders).Insert()
	if err != nil {
		log.Printf("error inserting orders: %v", err)
	}
	log.Printf("Inserted %d orders", len(orders))

	// Seed reviews
	reviews := make([]*Review, 10000)
	for i := range reviews {
		user := users[rand.Intn(len(users))]
		product := products[rand.Intn(len(products))]
		reviews[i] = &Review{
			ProductID: product.ID,
			UserID:    user.ID,
			Rating:    gofakeit.Number(1, 100),
			Content:   gofakeit.Paragraph(1, 3, 10, "."),
		}
	}
	_, err = db.Model(&reviews).Insert()
	if err != nil {
		log.Printf("error inserting reviews: %v", err)
	}
	log.Printf("Inserted %d reviews", len(reviews))

	return nil
}
