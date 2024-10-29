<p align="center">
  <a href="" rel="noopener">
 <img width=200px height=200px src="https://i.ibb.co/2tkXm7R/optivest-high-resolution-logo-transparent.png" alt="Project logo"></a>
</p>

<h3 align="center">OptiVest</h3>

<div align="center">

[![Status](https://img.shields.io/badge/status-active-success.svg)]()
[![GitHub Issues](https://img.shields.io/github/issues/Blue-Davinci/OptiVest.svg)](https://github.com/Blue-Davinci/OptiVest/issues)
[![GitHub Pull Requests](https://img.shields.io/github/issues-pr/Blue-Davinci/OptiVest.svg)](https://github.com/Blue-Davinci/OptiVest/pulls)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](/LICENSE)

</div>

---

<p align="center"> Optivest <b>[Back-End]: </b> This is the backend sister project of the OptiVest Project.
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

And repeat

```
until finished
```

End with an example of getting some data out of the system or using it for a little demo.

## 🔧 Running the tests <a name = "tests"></a>

Explain how to run the automated tests for this system.

### Break down into end to end tests

Explain what these tests test and why

```
Give an example
```

### And coding style tests

Explain what these tests test and why

```
Give an example
```

## 🎈 Usage <a name="usage"></a>

Add notes about how to use the system.

## 🚀 Deployment <a name = "deployment"></a>

Add additional notes about how to deploy this on a live system.

## ⛏️ Built Using <a name = "built_using"></a>

- [MongoDB](https://www.mongodb.com/) - Database
- [Express](https://expressjs.com/) - Server Framework
- [VueJs](https://vuejs.org/) - Web Framework
- [NodeJs](https://nodejs.org/en/) - Server Environment

## ✍️ Authors <a name = "authors"></a>

- [@Blue-Davinci](https://github.com/Blue-Davinci) - Idea & Initial work

See also the list of [contributors](https://github.com/Blue-Davinci/OptiVest/contributors) who participated in this project.

## 🎉 Acknowledgements <a name = "acknowledgement"></a>

- Hat tip to anyone whose code was used
- Inspiration
- References
