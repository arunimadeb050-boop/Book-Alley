package main

import (
	"fmt"
	"net/http"
)

func main() {
	connectDatabase() // opens bookshare.db and creates tables if needed
	defer db.Close()

	mux := http.NewServeMux()

	// ---- Auth & Profile ----
	mux.HandleFunc("POST /register", registerHandler)
	mux.HandleFunc("POST /login", loginHandler)
	mux.HandleFunc("GET /profile", myProfileHandler)
	mux.HandleFunc("PUT /profile", updateProfileHandler)
	mux.HandleFunc("GET /users/{id}", userProfileHandler)

	// ---- Books: browse, search, filter ----
	mux.HandleFunc("GET /books", getAllBooksHandler)
	mux.HandleFunc("GET /books/{id}", getOneBookHandler)
	mux.HandleFunc("POST /books", addBookHandler)
	mux.HandleFunc("PUT /books/{id}", updateBookHandler)
	mux.HandleFunc("DELETE /books/{id}", deleteBookHandler)
	mux.HandleFunc("GET /my/books", myBooksHandler)

	// ---- Borrow request & approval workflow ----
	mux.HandleFunc("POST /books/{id}/request", requestBookHandler)
	mux.HandleFunc("GET /requests/incoming", incomingRequestsHandler)
	mux.HandleFunc("GET /requests/outgoing", outgoingRequestsHandler)
	mux.HandleFunc("POST /requests/{id}/approve", approveRequestHandler)
	mux.HandleFunc("POST /requests/{id}/reject", rejectRequestHandler)
	mux.HandleFunc("POST /requests/{id}/return", returnRequestHandler)

	// ---- Cart & checkout (buying books) ----
	mux.HandleFunc("POST /cart/{bookId}", addToCartHandler)
	mux.HandleFunc("DELETE /cart/{bookId}", removeFromCartHandler)
	mux.HandleFunc("GET /cart", viewCartHandler)
	mux.HandleFunc("POST /checkout", checkoutHandler)
	mux.HandleFunc("GET /orders", myOrdersHandler)

	// ---- Ratings / reputation ----
	mux.HandleFunc("POST /users/{id}/review", addReviewHandler)

	fmt.Println("BookShare server running on http://localhost:8080")
	err := http.ListenAndServe(":8080", withCORS(mux))
	if err != nil {
		fmt.Println("server error:", err)
	}
}
