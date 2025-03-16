# GoLand Setup Guide for Homebox

## Initial Setup

1. **Install Required Tools**
   - [Go](https://golang.org/dl/) (version 1.23.0 or later)
   - [Task](https://taskfile.dev/installation/) - the task runner used in this project
     - Note: On Manjaro and some other Linux distributions, it's installed as `go-task` instead of `task`
   - [pnpm](https://pnpm.io/installation) - for frontend dependencies
   - [swag](https://github.com/swaggo/swag) - for API documentation

2. **Clone and Setup the Project**
   ```bash
   git clone https://github.com/sysadminsmedia/homebox.git
   cd homebox
   
   # On most systems:
   task setup
   
   # On Manjaro Linux:
   go-task setup
   ```

3. **Open in GoLand**
   - Open GoLand
   - Select "Open" and navigate to your cloned homebox directory
   - GoLand should automatically detect the Go module

## Backend Development (Go)

1. **Run the Backend Server**
   - Use the built-in Terminal in GoLand and run:
   ```bash
   # On most systems:
   task go:run
   
   # On Manjaro Linux:
   go-task go:run
   ```
   - The server will start at http://localhost:7745

2. **Run Go Tests**
   - Run all tests:
   ```bash
   # On most systems:
   task go:test
   
   # On Manjaro Linux:
   go-task go:test
   ```
   - Run a specific test:
   ```bash
   cd backend
   go test -v ./internal/data/repo -run TestUserRepo_GetOneEmail
   ```

3. **Lint Your Go Code**
   ```bash
   # On most systems:
   task go:lint
   
   # On Manjaro Linux:
   go-task go:lint
   ```

4. **Generate API Documentation**
   ```bash
   # On most systems:
   task swag
   
   # On Manjaro Linux:
   go-task swag
   ```

5. **Working with Database**
   - The project uses SQLite by default (simpler for development)
   - Database migrations are handled automatically
   - To generate new database models:
   ```bash
   # On most systems:
   task db:generate
   
   # On Manjaro Linux:
   go-task db:generate
   ```

## Frontend Development (Optional)

As you're focusing on the Go backend, you may not need to modify the frontend often, but here's how to run it:

1. **Start the Frontend Dev Server**
   ```bash
   # On most systems:
   task ui:dev
   
   # On Manjaro Linux:
   go-task ui:dev
   ```
   - This starts the Nuxt.js development server
   - Frontend will be available at http://localhost:3000

2. **Build the Frontend**
   ```bash
   cd frontend && pnpm build
   ```

## Working on Both Together

1. **Important: Run Both Frontend and Backend**
   
   Homebox uses a split architecture - the backend serves only the API and the frontend serves the UI.
   To work with the complete application, you need to run both:

   ```bash
   # Terminal 1: Start the backend server
   # On most systems:
   task go:run
   # On Manjaro Linux:
   go-task go:run
   
   # Terminal 2: Start the frontend dev server
   # On most systems:
   task ui:dev
   # On Manjaro Linux:
   go-task ui:dev
   ```

   - Backend API will be available at http://localhost:7745/api
   - Frontend UI will be available at http://localhost:3000

   If you only run the backend (`go-task go:run`), visiting localhost:7745 will just show a JSON error
   because there's no UI being served.

2. **Run Complete Integration Tests**
   ```bash
   # On most systems:
   task test:ci
   
   # On Manjaro Linux:
   go-task test:ci
   ```

3. **Prepare for a PR**
   - This runs all tests, linting, and generates required files:
   ```bash
   # On most systems:
   task pr
   
   # On Manjaro Linux:
   go-task pr
   ```

## Recommended Workflow for Go Developers

1. **Setup Your Admin User for Testing**
   - Edit the docker-compose.yml file to use your preferred admin credentials, or
   - Run the backend with admin user environment variables:
   ```bash
   # On Manjaro Linux:
   HBOX_ADMIN_CREATE=true HBOX_ADMIN_NAME=admin HBOX_ADMIN_EMAIL=admin@example.com HBOX_ADMIN_PASSWORD=changeme go-task go:run
   ```
   - In a separate terminal, run the frontend:
   ```bash
   # On Manjaro Linux:
   go-task ui:dev
   ```
   - Navigate to http://localhost:3000 in your browser
   - Log in with the admin credentials (admin@example.com / changeme)
   
   Note: The environment variables only create the admin user - you still need to log in through the frontend interface

2. **Focus on Backend Files**
   - `backend/app/api` - HTTP handlers and routes
   - `backend/internal/core/services` - Business logic 
   - `backend/internal/data` - Data access and database models

3. **Making Changes**
   - Modify Go code in GoLand
   - Run `go-task go:lint` (on Manjaro) to ensure code style 
   - Run `go-task go:test` (on Manjaro) to ensure tests pass
   - Restart the server with `go-task go:run` (on Manjaro) to see changes

4. **Debugging in GoLand**
   - Set breakpoints in your Go code
   - Create a Go Build configuration targeting `./backend/app/api`
   - Run in debug mode

## Useful Commands Quick Reference

For Manjaro Linux (using go-task):
- **Start Backend**: `go-task go:run`
- **Run Go Tests**: `go-task go:test`
- **Lint Go Code**: `go-task go:lint`
- **Generate Database Models**: `go-task db:generate`
- **Run Frontend (if needed)**: `go-task ui:dev`

For other systems (using task):
- **Start Backend**: `task go:run`
- **Run Go Tests**: `task go:test`
- **Lint Go Code**: `task go:lint`
- **Generate Database Models**: `task db:generate`
- **Run Frontend (if needed)**: `task ui:dev`

Remember, you can always check `Taskfile.yml` for all available commands, and you can find configuration details in `/docs/en/configure.md`.

## Docker Development

If you prefer to use Docker for development:

1. **Run with Docker Compose**
   ```bash
   docker-compose up
   ```

2. **Access the Application**
   - Frontend will be available at http://localhost:3100
   - API will be available at http://localhost:3100/api

3. **Admin User Configuration**
   The docker-compose.yml is configured to:
   - Disable self-registration
   - Create an admin user on first startup with:
     - Email: admin@example.com
     - Password: changeme
   - Data is persisted in a Docker volume

4. **Making Changes**
   - After code changes, rebuild the Docker image:
   ```bash
   docker-compose build
   docker-compose up
   ```

## Database Management

1. **Database Location**
   
   By default, Homebox uses SQLite with the database file. **Important**: The actual database location is:
   ```
   backend/.data/homebox.db
   ```
   
   This is different from what's configured in the `Taskfile.yml`:
   ```yaml
   HBOX_DATABASE_DRIVER: sqlite3
   HBOX_DATABASE_SQLITE_PATH: .data/homebox.db?_pragma=busy_timeout=1000&_pragma=journal_mode=WAL&_fk=1&_time_format=sqlite
   ```
   
   The discrepancy is because:
   - When running the backend directly with `go-task go:run`, the working directory is `backend/`
   - The database path `.data/homebox.db` is relative to that working directory
   - So the database is created at `backend/.data/homebox.db`, not at the project root
   
   Important notes:
   - The database file is created automatically when the application starts
   - Admin users are created in this database at `backend/.data/homebox.db`
   - The application creates the `backend/.data` directory automatically (line 143 in main.go)
   - That's why you might see "admin user already exists" messages even when you don't see a database file in the project root

2. **Resetting the Database**

   To reset the database, you can simply stop the running application and delete the database file:
   ```bash
   # Stop any running instances first
   rm -f backend/.data/homebox.db*
   ```
   The database will be recreated automatically when you restart the application.

3. **Connecting to the Database in GoLand**

   To connect to the SQLite database in GoLand:
   
   a. Using built-in Database Tools:
      - Click on the "Database" tool window on the right side
      - Click the "+" button and select "Data Source" → "SQLite"
      - For the file path, browse to your project's `backend/.data/homebox.db` file
      - Test the connection and click "Apply" then "OK"
   
   b. If the built-in tool doesn't work, install the "Database Navigator" plugin:
      - Go to File → Settings → Plugins
      - Search for "Database Navigator"
      - Install and restart GoLand if needed
      - Add a connection through the Database Navigator panel
   
   c. Now you can browse tables, run SQL queries, and view/edit data directly in GoLand
   
   d. You can also create a Database Run Configuration:
      - Go to Run → Edit Configurations
      - Click + and select "Database Script"
      - Choose your SQLite connection
      - Create SQL queries to run against the database

4. **Viewing Database Structure**

   For a quick view of the database schema using the command line:
   ```bash
   sqlite3 backend/.data/homebox.db .schema
   ```
   
   Or examine the schema definitions in code:
   ```
   backend/internal/data/ent/schema/
   ```

5. **Using PostgreSQL Instead of SQLite**

   Homebox also supports PostgreSQL if you prefer a more robust database:
   
   a. Start a PostgreSQL server (via Docker or your system's package manager)
   
   b. Run the backend with PostgreSQL configuration:
   ```bash
   # On Manjaro:
   go-task go:run:postgresql
   ```
   
   Or set these environment variables manually:
   ```bash
   export HBOX_DATABASE_DRIVER=postgres
   export HBOX_DATABASE_USERNAME=homebox
   export HBOX_DATABASE_PASSWORD=homebox
   export HBOX_DATABASE_DATABASE=homebox
   export HBOX_DATABASE_HOST=localhost
   export HBOX_DATABASE_PORT=5432
   export HBOX_DATABASE_SSL_MODE=disable
   go-task go:run
   ```
   
   c. Connect to PostgreSQL in GoLand:
      - Click on the Database tool window
      - Add a PostgreSQL connection using the credentials above
      - Test and apply the connection

6. **Database Migrations**
   
   Database migrations are handled automatically. The SQL migration files are located at:
   ```
   backend/internal/data/migrations/sqlite3/  # SQLite migrations
   backend/internal/data/migrations/postgres/ # PostgreSQL migrations
   ```
   
   To generate a new migration:
   ```bash
   # For SQLite (on Manjaro):
   go-task db:migration [migration_name]
   
   # For PostgreSQL (on Manjaro):
   go-task db:migration:postgresql [migration_name]
   ```

7. **Verifying Database Creation**

   To verify the database is being created correctly:
   
   a. Start both backend and frontend:
   ```bash
   # Terminal 1
   go-task go:run
   
   # Terminal 2
   go-task ui:dev
   ```
   
   b. Access the web interface at http://localhost:3000
   
   c. Register a new user or login (this triggers database creation)
   
   d. Check if the database file exists:
   ```bash
   ls -la backend/.data/homebox.db*
   ```
   
   e. If you want to watch database creation in real-time:
   ```bash
   # In a separate terminal
   watch -n 1 "ls -la backend/.data/"
   ```
   
   f. For detailed database debugging, set the log level to debug:
   ```bash
   HBOX_LOG_LEVEL=debug go-task go:run
   ```

## Troubleshooting

1. **JSON Error at localhost:7745**
   
   If you see `{"error":"Unknown Error"}` when visiting http://localhost:7745, this is normal.
   The backend only serves the API, not the UI. You need to:
   
   - Run the frontend separately with `go-task ui:dev` (on Manjaro)
   - Access the application at http://localhost:3000

2. **Authorization Header Error**

   If you see an error like:
   ```
   ERR internal/web/mid/errors.go:31 > ERROR occurred error="authorization header or query is required"
   ```
   
   This is normal when you:
   - Try to access protected API endpoints directly
   - Access the API without a valid authentication token
   
   To resolve this:
   - Make sure you're accessing the application through the frontend (http://localhost:3000)
   - The frontend will handle authentication properly
   - Avoid accessing backend API endpoints directly without authentication
   
   Note: Understanding admin user creation process:
   - Using `HBOX_ADMIN_CREATE=true` and related variables creates the admin user in the database during startup
   - However, this only creates the user - it doesn't authenticate you
   - You still need to:
     1. Run both backend and frontend
     2. Go to the frontend login page (http://localhost:3000)
     3. Log in with the admin credentials you set up (email: admin@example.com, password: changeme)
     4. After logging in, you'll have full access to the application

3. **Database Issues**
   
   If you encounter database errors, make sure the `.data` directory exists:
   
   ```bash
   mkdir -p .data
   ```

4. **Port Already in Use**
   
   If port 7745 or 3000 is already in use, you can modify the ports in:
   - Backend: Set environment variable `HBOX_WEB_PORT=xxxx` 
   - Frontend: Edit `frontend/nuxt.config.ts`

5. **Frontend Dependencies**
   
   If you encounter errors with frontend dependencies:
   
   ```bash
   cd frontend
   pnpm install
   ```