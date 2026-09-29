# BookShare Backend (Go)

A backend for a book lending + buying platform. Covers:
- Secure registration, login (JWT), and profiles
- List / search / filter books by genre, author, price, availability
- Request-and-approval workflow for borrowing, with due-date tracking
- Cart and checkout for buying books
- Rating/reputation system

Data is stored permanently in a SQLite file (`bookshare.db`), not memory, so it survives restarts.

## Setup

You need Go 1.22+ installed (`go version` to check) and an internet connection the first time
(to download 3 small libraries).

```bash
cd bookshare
go mod tidy      # downloads jwt, bcrypt, and the sqlite driver
go run .
```

You should see: `BookShare server running on http://localhost:8080`

A file called `bookshare.db` will appear in the folder — that's your database. Delete it anytime to start fresh.

## Files in this project

| File          | What's inside                                          |
|---------------|----------------------------------------------------------|
| `models.go`   | The shape of our data (User, Book, BorrowRequest, Order, Review) |
| `database.go` | Connects to SQLite and creates the tables               |
| `helpers.go`  | Small reusable functions: JSON, JWT, CORS                |
| `auth.go`     | Register, login, profile, reputation                     |
| `books.go`    | List, search, filter, add, edit, delete books             |
| `requests.go` | Borrow request → approve/reject → return, with due dates  |
| `cart.go`     | Cart, checkout, order history                             |
| `reviews.go`  | Star ratings after a completed exchange                   |
| `main.go`     | Connects every URL to its handler and starts the server   |

## All endpoints

Send `Authorization: Bearer <token>` for anything marked "login needed".

**Auth & profile**
| Method | URL | |
|---|---|---|
| POST | /register | |
| POST | /login | |
| GET | /profile | login needed — my profile |
| PUT | /profile | login needed — edit name/bio/city |
| GET | /users/{id} | anyone's public profile + rating |

**Books**
| Method | URL | |
|---|---|---|
| GET | /books | filters: search, genre, author, min_price, max_price, available, type |
| GET | /books/{id} | |
| POST | /books | login needed |
| PUT | /books/{id} | login needed, owner only |
| DELETE | /books/{id} | login needed, owner only |
| GET | /my/books | login needed |

**Borrowing**
| Method | URL | |
|---|---|---|
| POST | /books/{id}/request | login needed — ask to borrow |
| GET | /requests/incoming | login needed — requests for my books |
| GET | /requests/outgoing | login needed — my requests, shows due dates |
| POST | /requests/{id}/approve | login needed, owner only. Body: `{"days":7}` optional |
| POST | /requests/{id}/reject | login needed, owner only |
| POST | /requests/{id}/return | login needed, owner confirms return |

**Buying**
| Method | URL | |
|---|---|---|
| POST | /cart/{bookId} | login needed |
| DELETE | /cart/{bookId} | login needed |
| GET | /cart | login needed |
| POST | /checkout | login needed |
| GET | /orders | login needed — my purchase history |

**Reviews**
| Method | URL | |
|---|---|---|
| POST | /users/{id}/review | login needed, only after a completed exchange |

## Test the whole flow with curl

```bash
# Register Rahul and Priya
curl -X POST localhost:8080/register -d '{"name":"Rahul","email":"rahul@mail.com","password":"secret123"}'
curl -X POST localhost:8080/register -d '{"name":"Priya","email":"priya@mail.com","password":"secret123"}'

# Login as each, save their tokens
curl -X POST localhost:8080/login -d '{"email":"rahul@mail.com","password":"secret123"}'
curl -X POST localhost:8080/login -d '{"email":"priya@mail.com","password":"secret123"}'

# Rahul lists a book, for both lending and selling
curl -X POST localhost:8080/books -H "Authorization: Bearer RAHUL_TOKEN" \
  -d '{"title":"Dune","author":"Frank Herbert","genre":"Sci-Fi","description":"Desert planet politics","for_lend":true,"for_sale":true,"price":300}'

# Browse and filter
curl "localhost:8080/books?genre=sci-fi"
curl "localhost:8080/books?max_price=500&type=sale"

# --- Borrowing flow ---
curl -X POST localhost:8080/books/1/request -H "Authorization: Bearer PRIYA_TOKEN"
curl localhost:8080/requests/incoming -H "Authorization: Bearer RAHUL_TOKEN"
curl -X POST localhost:8080/requests/1/approve -H "Authorization: Bearer RAHUL_TOKEN" -d '{"days":10}'
curl -X POST localhost:8080/requests/1/return -H "Authorization: Bearer RAHUL_TOKEN"

# --- Buying flow ---
curl -X POST localhost:8080/cart/1 -H "Authorization: Bearer PRIYA_TOKEN"
curl localhost:8080/cart -H "Authorization: Bearer PRIYA_TOKEN"
curl -X POST localhost:8080/checkout -H "Authorization: Bearer PRIYA_TOKEN"

# --- Rating (only works after the exchange above) ---
curl -X POST localhost:8080/users/1/review -H "Authorization: Bearer PRIYA_TOKEN" \
  -d '{"rating":5,"comment":"Book was in great condition!"}'
curl localhost:8080/users/1
```

## Notes for your report / viva

- **Passwords**: hashed with bcrypt, never stored in plain text.
- **Authentication**: JWT tokens, valid 24 hours, checked on every protected route.
- **SQL injection protection**: every query uses `?` placeholders instead of pasting values into the SQL string.
- **Transactions**: approving a request, returning a book, and checkout all touch two+ tables together — wrapped in `db.Begin()/Commit()/Rollback()` so a crash can't leave half-finished data (e.g. an order created but books not marked sold).
- **Frontend technologies expected by your objectives (HTML, CSS, JS)**: this backend exposes plain JSON over HTTP, so any frontend (plain JS `fetch`, or a framework) can consume it the same way.

## Ideas if you want to go further

- Password reset via email
- Image upload for book covers
- Pagination on `/books` for large catalogs
- Admin role to remove abusive listings
