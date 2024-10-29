<p align="center">
  <a href="" rel="noopener">
 <img width=300px height=190px src="https://i.ibb.co/hZdMWvh/optivest-cropped.png" alt="Project logo"></a>
</p>

<h3 align="center">OptiVest</h3>

<div align="center">

[![Status](https://img.shields.io/badge/status-active-success.svg)]()
[![GitHub Issues](https://img.shields.io/github/issues/Blue-Davinci/OptiVest.svg)](https://github.com/Blue-Davinci/OptiVest/issues)
[![GitHub Pull Requests](https://img.shields.io/github/issues-pr/Blue-Davinci/OptiVest.svg)](https://github.com/Blue-Davinci/OptiVest/pulls)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](/LICENSE)

</div>

---

<p align="center"> Optivest <b>[Back-End]: </b> This is the backend sister project of OptiVest Project. To Check out the Frontend, go <a href="https://github.com/Blue-Davinci/OptiVest-Frontend">here</a>

    <br> 
</p>

## 📝 Table of Contents

- [About](#about)
- [Features](#features)
- [Getting Started](#getting_started)
- [Deployment](#deployment)
- [Usage](#usage)
- [Built Using](#built_using)
- [TODO](../TODO.md)
- [Contributing](../CONTRIBUTING.md)
- [Authors](#authors)
- [Acknowledgments](#acknowledgement)

## 🧐 About <a name = "about"></a>

The OptiVest project is a cutting-edge, AI-driven personal financial advisor platform designed to empower users with smarter financial management tools. Built with a focus on automation and real-time data insights, OptiVest combines dynamic portfolio analysis, personalized investment recommendations, and a suite of tools for budgeting, goal setting, and debt tracking. The backend, developed in Go, integrates financial data from sources like Alpha Vantage and FRED to provide up-to-date insights and robust portfolio optimization.

A standout feature of OptiVest is its focus on actionable financial insights. Users receive real-time portfolio alerts, performance metrics, and risk management tips, helping them make well-informed decisions. The platform’s intelligent algorithms highlight top-performing assets and assist in sector diversification, while additional tools for budgeting and debt tracking offer a holistic approach to personal finance. By merging AI-driven recommendations with user-centric design, OptiVest delivers an all-in-one financial advisory experience tailored to individual financial goals and preferences.

## ✨ Features <a name="features"></a>
1. **AI-Driven Financial Insights**
- Provides intelligent financial advice using pre-trained AI models, enabling users to make data-backed investment decisions.
- Customizable recommendations on portfolio rebalancing, risk management, and asset allocation.
2. **Real-Time Portfolio Analysis**
- Integrates with Alpha Vantage and FRED for up-to-date data, delivering real-time analysis of investments, market trends, and external factors like interest rates and market sentiment.
- Calculates key performance metrics such as ROI, Sharpe ratio, and sector performance.
3. **Automated Portfolio Management**
- Supports automated portfolio rebalancing based on individual risk tolerance and investment goals.
- Uses advanced algorithms to identify top-performing stocks and bonds, updating recommendations regularly.
4. **Personal Finance Tools**
- Budgeting and Goal Setting: Tracks spending, monitors goals, and provides summaries for financial planning.
- Debt Management: Analyzes debt information, including payment history, interest rates, and payoff estimates, and visualizes debt progress.
5. **Notification Center**
- Real-time notifications for market updates, investment alerts, and goal progress. **In Progress**
- Allows users to view messages with detailed metadata, including links and images, for quick navigation.
6. **Advanced Security and Integration**
- Secure WebSocket connection for real-time updates and data handling.
- Implements Redis caching for efficient data retrieval, reducing load on API calls and improving performance.
7. **Prediction Capability**
- Based on your spending, expense, income and debt rates, OptiVest is able to come up with predictions of future habits
using the OptiVest Predictor Micr-Service.

## 🏁 Getting Started <a name = "getting_started"></a>

These instructions will get you a copy of the project up and running on your local machine for development and testing purposes. See [deployment](#deployment) for notes on how to deploy the project on a live system.

### Prerequisites

Before you can run or contribute to this project, you'll need to have the following software installed:

- [Go](https://golang.org/dl/): The project is written in Go, so you'll need to have Go installed to run or modify the code.
- [PostgreSQL](https://www.postgresql.org/download/): The project uses a PostgreSQL database, so you'll need to have PostgreSQL installed and know how to create a database.
- A Go IDE or text editor: While not strictly necessary, a Go IDE or a text editor with Go support can make it easier to work with the code. I use vscode.
- [Git](https://git-scm.com/downloads): You'll need Git to clone the repo.
- [Redis](https://redis.io/): OptiVest uses Redis for caching to enhance performance and reduce API load.
- [OptiVest-Predictor-Microservice](https://github.com/Blue-Davinci/OptiVest_Finance_Predictor_Micro_Service_V1): Clone and ser up this micro-service, which is esential for financial predictions and recommendations
```
Give examples
```

### Installing

A step by step series of examples that tell you how to get a development env running.

1. **Clone the repository:** Start by cloning the repository to your local machine. Open a terminal, navigate to the directory where you want to clone the repository, and run the following command:
    ```bash
    git clone https://github.com/Blue-Davinci/OptiVest.git
    ```
2. **Navigate to the project directory:** Use the `cd` command to navigate to the project directory:

    ```bash
    cd optivest
    ```
3. **Install the Go dependencies:** The Go tools will automatically download and install the dependencies listed in the `go.mod` file when you build or run the project. To download the dependencies without building or running the project, you can use the `go mod download` command:

    ```bash
    go mod download
    ```
4. **Set up the database:** The project uses a PostgreSQL database. You'll need to create a new database and update the connection string in your configuration file or environment variables.
We use `GOOSE` for all the data migrations and `SQLC` as the abstraction layer for the DB. To proceed
with the migration, navigate to the `Schema` director:
```bash
cd internal\sql\schema
```
- Then proceed by using the `goose {connection string} up` to execute an <b>Up migration</b> as shown:
- <b>Note:</b> You can use your own environment variable or load it from the env file.

```bash
goose postgres postgres://aggregate:password@localhost/aggregate  up
```

5. **Download and Setup the MIcroService:** Follow the instructions highlighted [here](https://github.com/Blue-Davinci/OptiVest_Finance_Predictor_Micro_Service_V1) to get the micro-service up and running.

6. **Environment Variable Setups:** OptiVest uses a few external APIs. You will need to set them up and make an `.env` containing the following templates:
```bash
# DSN Link to our Postgres database
OPTIVEST_DB_DSN=postgres://optivest:yourpassword@localhost/optivest?sslmode=disable
# For deployment: , comment the above and uncomment the below DSN
#OPTIVEST_DB_DSN=postgres://optivest:yourpassword@host.docker.internal/optivest?sslmode=disable
OPTIVEST_DATA_ENCRYPTION_KEY=xxxxxxx

# Mailer configuration
OPTIVEST_SMTP_HOST=xxxxx
OPTIVEST_SMTP_USERNAME=xxxxxx
OPTIVEST_SMTP_PASSWORD=xxxxxxx
OPTIVEST_SMTP_SENDER=Optivest <no-reply@optivest.tech>

# Exchange Rate API
OPTIVEST_EXCHANGERATE_API_KEY=xxxxxxxxx
# Alpha Vantage API
OPTIVEST_ALPHAVANTAGE_API_KEY=xxxxxx
# Fred API
OPTIVEST_FRED_API_KEY=xxx
# Financial Modeling Prep API
OPTIVEST_FINANCIALMODELINGPREP_API_KEY=xxxxx
# Samba Nova LLM API
OPTIVEST_SAMBA_NOVA_LLM_API_KEY=xxxx
# Optivest Predictor Microservice
OPTIVEST_PREDICTOR_API_KEY=xxx
# OCR.Space API
OPTIVEST_OCRSPACE_API_KEY=xxxx
```
**The above .env is self explanatory for each API needed**

5. **Build the project:** You can build the project using the makefile's command:

    ```bash
    make build/api
    ```
    This will create an executable file in the current directory.
    <b>Note: The generated executable is for the windows environment</b>
      <b>- However, You can find the linux build command within the makefile!</b>

6. **Run the project:** You can run the project using the `go run` or use <b>`MakeFile`</b> and do:

    ```bash
    make run/api
    ```

7. **MakeFile Help:** For additional supported commands run `make help`:

  ```makefile
  make help
  ```
**output:**
```bash
Usage:
run/api: Run the API server
run/api/origins: Run the API server with CORS origins
db/psql            -  connect to the db using psql
build/api          -  build the cmd/api application
```

### Description

The application accepts command-line flags for configuration, establishes a connection pool to a database, and publishes variables for monitoring the application. The published variables include the application version, the number of active goroutines and the current Unix timestamp.
  - This will start the application. You should be able to access it at `http://localhost:4000`.

<hr />
You can view the **parameters** by utilizing the `-help` command. Here is a rundown of 
the available commands for a quick lookup (INCOMPLETE, use help for full list).

```bash
- api-author string
        API author (default "Blue_Davinci")
  -api-default-currency string
        Default currency (default "USD")
  -api-key-alphavantage string
        Alpha Vantage API key (default "NYRXRLGLWY29115K")
  -api-key-exchangerates string
        Exchange-Rate API Key (default "2bd6d65a467e533704e0a7fb")
  -api-key-fmp string
        FMP API Key (default "LtEBinivcBs2uzHNg1PHXJIRj5KsLmxJ")
  -api-key-fred string
        FRED API Key (default "1c78299c9778f33eeacf2f85261f9183")
  -api-key-ocrspace string
        OCR.Space API Key (default "K82853087188957")
  -api-key-optivestmicroservice string
        OptiVest Microservice API Key (default "xJ8u4Kz7wA9vT3gPzB2dF1mLq8N5cY6s")
  -api-key-sambanova string
        Sambanova API Key (default "4044b951-1161-4165-b025-8a7bc6f46155")
  -api-name string
        API name (default "OptiVest")
  -api-url-alphavantage string
        Alpha Vantage API URL (default "https://www.alphavantage.co/query?")
  -api-url-exchangerates string
        Exchange-Rate API URL (default "https://v6.exchangerate-api.com/v6")
  -api-url-fmp string
        FMP API URL (default "https://financialmodelingprep.com/api/v3")
  -api-url-fred string
        FRED API URL (default "https://api.stlouisfed.org/fred/series/observations?")
  -api-url-ocrspace string
        OCR.Space API URL (default "https://api.ocr.space/parse/image")
  -api-url-optivestmicroservice string
        OptiVest Microservice API URL (default "http://127.0.0.1:8000/v1/predict")
  -api-url-sambanova string
        Sambanova API URL (default "https://fast-api.snova.ai/v1/chat/completions")
  -cors-trusted-origins value
        Trusted CORS origins (space separated)
  -db-dsn string
        PostgreSQL DSN (default "postgres://optivest:pa55word@localhost/optivest?sslmode=disable")
  -db-max-idle-conns int
        PostgreSQL max idle connections (default 25)
  -db-max-idle-time string
        PostgreSQL max connection idle time (default "15m")
  -db-max-open-conns int
        PostgreSQL max open connections (default 25)
  -encryption-key string
        Encryption key (default "330d12d2eacd444bd87126221c91150a09cbaee8529a387282e1e910c8be3868")
  -env string
        Environment (development|staging|production) (default "development")
  -expired-notification-burst-limit int
        Batch Limit for Expired Notification Tracker (default 100)
  -frontend-activation-url string
        Frontend Activation URL (default "http://localhost:5173/verify?token=")
  -frontend-callback-url string
        Frontend Callback URL (default "https://adapted-healthy-monitor.ngrok-free.app/v1")
  -frontend-login-url string
        Frontend Login URL (default "http://localhost:5173/login")
  -frontend-password-reset-url string
        Frontend Password Reset URL (default "http://localhost:5173/passwordreset/password?token=")
  -frontend-url string
        Frontend URL (default "http://localhost:5173")
  -http-client-retrymax int
        HTTP client maximum retries (default 3)
  -http-client-timeout duration
        HTTP client timeout (default 10s)
```

Using `make run`, will run the API with a default connection string located 
in `cmd\api\.env`. If you're using `powershell`, you need to load the values otherwise you will get
a `cannot load env file` error. Use the PS code below to load it or change the env variable:

```powershell
$env:OPTIVEST_DB_DSN=(Get-Content -Path .\cmd\api\.env | Where-Object { $_ -match "OPTIVEST_DB_DSN" } | ForEach-Object { $($_.Split("=", 2)[1]) })
```

Alternatively, in unix systems you can make a .envrc file and load it directly in the makefile by importing like so:
```makefile
include .envrc
```

A succesful run will output something like this:

```bash
make run/api
Running API server..
go run ./cmd/api
{"level":"info","time":"2024-1","caller":"api/main.go:374","msg":"Loading Environment Variables","path":"cmd\\api\\.env"}
{"level":"info","time":"2024-1]","caller":"api/main.go:268","msg":"Redis connection established","addr":"localhost:6379"}
{"level":"info","time":"2024-1","caller":"api/main.go:277","msg":"database connection pool established","dsn":"postgres://optivest:yourpassword@localhost/optivest?sslmode=disable"}
{"level":"info","time":"2024-1","caller":"api/helpers.go:215","msg":"currency certified, using cached currencies","currency":"USD"}
{"level":"info","time":"2024-1-2","caller":"api/main.go:337","msg":"Starting Schedulers"}
{"level":"info","time":"2024-0-2","caller":"api/schedulers.go:64","msg":"Starting the recurring expenses tracking cron job..","time":"2024-00-2"}
{"level":"info","time":"2024-1","caller":"api/schedulers.go:199","msg":"Tracking recurring expenses","time":"2024-1 EAT m=+0.533360001"}
```

<hr />

### API Endpoints <a name = "endpoints"></a>
A full ist is documented using swagger, but here is a quick runwdown:
1. **GET /v1/healthcheck:** Checks the health of the application. Returns a 200 OK status code if the application is running correctly.

2. **POST /v1/users:** Registers a new user.

3. **PUT /v1/users/activated:** Activates a user.

4. **POST /v1/api/authentication:** Creates an authentication token.

5. **GET /debug/vars:** Provides debug variables from the `expvar` package. 


## 🔧 Running the tests <a name = "tests"></a>

The project has existing tests represented by files ending with the word `"_test"` e.g `internal_helpers_test.go`

### Break down into end to end tests

Each test file contains a myriad of tests to run on various entities mainly functions.
The test files are organized into `structs of tests` and their corresponding test logic.

You can run them directly from the vscode test UI. Below represents test results for the scraper:

```bash
=== RUN   Test_generateSecurityKey
=== RUN   Test_generateSecurityKey/Valid_AES-128_key_(16_bytes)
--- PASS: Test_generateSecurityKey/Valid_AES-128_key_(16_bytes) (0.00s)
=== RUN   Test_generateSecurityKey/Valid_AES-192_key_(24_bytes)
--- PASS: Test_generateSecurityKey/Valid_AES-192_key_(24_bytes) (0.00s)
=== RUN   Test_generateSecurityKey/Valid_AES-256_key_(32_bytes)
--- PASS: Test_generateSecurityKey/Valid_AES-256_key_(32_bytes) (0.00s)
=== RUN   Test_generateSecurityKey/Invalid_key_length_(0_bytes)
--- PASS: Test_generateSecurityKey/Invalid_key_length_(0_bytes) (0.00s)
=== RUN   Test_generateSecurityKey/Invalid_key_length_(-1_bytes)
--- PASS: Test_generateSecurityKey/Invalid_key_length_(-1_bytes) (0.00s)
--- PASS: Test_generateSecurityKey (0.00s)
PASS
ok      github.com/Blue-Davinci/OptiVest/internal/data  0.674s
```
- <b>All other tests follow a similar prologue.</b>

## 🎈 Usage <a name="usage"></a>
As earlier mentioned, the api uses a myriad of flags which you can use to launch the application.
An example of launching the application with your `smtp server's setting` includes:
```bash
make build/api ## build api using the makefile
./bin/api.exe -smtp-username=pigbearman -smtp-password=algor ## run the built api with your own values

Direct Run: 
go run main.go
```


## 🚀 Deployment <a name = "deployment"></a>

**(Will Be Added Soon.)**

## ⛏️ Built Using <a name = "built_using"></a>
- [Go](https://golang.org/) - Backend
- [PostgreSQL](https://www.postgresql.org/) - Database
- [Redis](https://redis.io/) - Cacheing
- [Paystack](https://paystack.com) - Payment processing
- [SQLC](https://github.com/kyleconroy/sqlc) - Generate type safe Go from SQL
- [Goose](https://github.com/pressly/goose) - Database migration tool
- [HTML/CSS](https://developer.mozilla.org/en-US/docs/Web/HTML) - Templates

## ✍️ Authors <a name = "authors"></a>

- [@Blue-Davinci](https://github.com/Blue-Davinci) - Idea & Initial work

See also the list of [contributors](https://github.com/Blue-Davinci/OptiVest/contributors) who participated in this project.

## 🎉 Acknowledgements <a name = "acknowledgement"></a>

- Hat tip to anyone whose code was used
- Inspiration

## 📚 References <a name = "references"></a>

- [Go Documentation](https://golang.org/doc/): Official Go documentation and tutorials.
- [PostgreSQL Documentation](https://www.postgresql.org/docs/): Official PostgreSQL documentation.
- [SQLC Documentation](https://docs.sqlc.dev/en/latest/): Official SQLC documentation and guides.
