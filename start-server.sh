#!/bin/bash
# Dashboard Generator Server Startup Script
# Loads .env file and starts the server

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Check if .env exists
if [ ! -f .env ]; then
    echo -e "${YELLOW}⚠️  No .env file found${NC}"
    echo "Creating .env from template..."
    if [ -f .env.example ]; then
        cp .env.example .env
        echo -e "${GREEN}✓ Created .env from .env.example${NC}"
        echo -e "${YELLOW}⚠️  Please edit .env and add your ANTHROPIC_API_KEY${NC}"
        echo ""
    else
        echo -e "${RED}✗ .env.example not found${NC}"
        exit 1
    fi
fi

# Load environment variables
echo "Loading environment variables from .env..."
export $(grep -v '^#' .env | grep -v '^$' | xargs)

# Check for required binary
if [ ! -f ./dashboard-generator ]; then
    echo -e "${YELLOW}⚠️  Binary not found. Building...${NC}"
    make build
fi

# Check if API key is set
if [ -z "$ANTHROPIC_API_KEY" ] || [ "$ANTHROPIC_API_KEY" = "sk-ant-api03-your-key-here" ]; then
    echo -e "${YELLOW}⚠️  ANTHROPIC_API_KEY not configured${NC}"
    echo "   AI features will be disabled"
    echo "   Get your key from: https://console.anthropic.com/"
    echo ""
else
    echo -e "${GREEN}✓ AI features enabled (Claude API key found)${NC}"
fi

# Set defaults
PORT=${PORT:-8080}
CONFIG=${CONFIG:-example-config.yaml}

# Start server
echo ""
echo -e "${GREEN}🚀 Starting Dashboard Generator...${NC}"
echo "   Port: $PORT"
echo "   Config: $CONFIG"
if [ -n "$GRAFANA_URL" ]; then
    echo "   Grafana: $GRAFANA_URL"
fi
echo ""
echo -e "${GREEN}📍 Access at: http://localhost:$PORT${NC}"
echo ""
echo "Press Ctrl+C to stop"
echo ""

./dashboard-generator serve \
    --config "$CONFIG" \
    --port "$PORT" \
    ${GRAFANA_URL:+--grafana-url "$GRAFANA_URL"}
