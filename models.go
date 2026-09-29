package main

// This file only describes WHAT our data looks like.
// Each struct matches one database table (or one JSON response).
// `json:"..."` sets the field name in JSON. `json:"-"` hides a field.

// User = a person on the platform (a table row in "users")
type User struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email,omitempty"` // omitempty = skip if empty
	Password string `json:"-"`               // bcrypt hash, never sent to the client
	Bio      string `json:"bio"`
	City     string `json:"city"`
}

// Book = a book someone listed.
// A book can be lent, sold, or both.
// Status is one of: "available", "borrowed", "sold"
type Book struct {
	ID          int     `json:"id"`
	Title       string  `json:"title"`
	Author      string  `json:"author"`
	Genre       string  `json:"genre"`
	Description string  `json:"description"`
	OwnerID     int     `json:"owner_id"`
	ForLend     bool    `json:"for_lend"`
	ForSale     bool    `json:"for_sale"`
	Price       float64 `json:"price"`
	Status      string  `json:"status"`
}

// BookInput = what the client sends when adding/editing a book
type BookInput struct {
	Title       string  `json:"title"`
	Author      string  `json:"author"`
	Genre       string  `json:"genre"`
	Description string  `json:"description"`
	ForLend     bool    `json:"for_lend"`
	ForSale     bool    `json:"for_sale"`
	Price       float64 `json:"price"`
}

// BorrowRequest = "I would like to borrow your book"
// Status flow:  pending -> approved -> returned
//               pending -> rejected
type BorrowRequest struct {
	ID          int    `json:"id"`
	BookID      int    `json:"book_id"`
	BookTitle   string `json:"book_title"`
	BorrowerID  int    `json:"borrower_id"`
	OwnerID     int    `json:"owner_id"`
	Status      string `json:"status"`
	RequestedAt string `json:"requested_at"`
	DueDate     string `json:"due_date"`    // set when approved
	ReturnedAt  string `json:"returned_at"` // set when returned
	Overdue     bool   `json:"overdue"`     // calculated, not stored
}

// OrderItem = one book inside an order
type OrderItem struct {
	BookID   int     `json:"book_id"`
	Title    string  `json:"title"`
	Price    float64 `json:"price"`
	SellerID int     `json:"seller_id"`
}

// Order = one completed checkout
type Order struct {
	ID        int         `json:"id"`
	Total     float64     `json:"total"`
	CreatedAt string      `json:"created_at"`
	Items     []OrderItem `json:"items"`
}

// Review = a rating (1-5 stars) one user gives another
type Review struct {
	Rating       int    `json:"rating"`
	Comment      string `json:"comment"`
	ReviewerName string `json:"reviewer_name"`
	CreatedAt    string `json:"created_at"`
}
