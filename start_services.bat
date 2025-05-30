@echo off
echo =============================================
echo       Starting All gRPC Services
echo =============================================
echo.

echo Starting services on different ports...
echo.

echo Starting User Service (Port 8001)...
start "User-Service-8001" cmd /k "cd services/user-service && go run main.go 8001 v1.0"

echo Starting User Service (Port 8002)...
start "User-Service-8002" cmd /k "cd services/user-service && go run main.go 8002 v1.0"

echo Starting Order Service (Port 8003)...
start "Order-Service-8003" cmd /k "cd services/order-service && go run main.go 8003 v1.0"

echo Starting Order Service (Port 8004)...
start "Order-Service-8004" cmd /k "cd services/order-service && go run main.go 8004 v1.0"

echo Starting Payment Service (Port 8005)...
start "Payment-Service-8005" cmd /k "cd services/payment-service && go run main.go 8005 v1.0"

echo Starting Payment Service (Port 8006)...
start "Payment-Service-8006" cmd /k "cd services/payment-service && go run main.go 8006 v1.0"

echo.
echo All services are starting...
echo.
echo Service Information:
echo   User Service:    localhost:8001, localhost:8002
echo   Order Service:   localhost:8003, localhost:8004  
echo   Payment Service: localhost:8005, localhost:8006
echo.
echo Health Check URLs:
echo   User Service:    http://localhost:9001/health, http://localhost:9002/health
echo   Order Service:   http://localhost:9003/health, http://localhost:9004/health
echo   Payment Service: http://localhost:9005/health, http://localhost:9006/health
echo.
echo Wait a few seconds for services to start, then run:
echo   go run production_example.go
echo.
echo Press any key to continue...
pause > nul 