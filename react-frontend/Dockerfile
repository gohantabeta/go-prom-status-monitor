# react-frontend/Dockerfile

# --- ビルドステージ ---
FROM node:18 AS builder

WORKDIR /app

COPY package*.json ./
RUN npm install

COPY . .
RUN npm run build

# --- 実行ステージ ---
FROM nginx:stable-alpine

COPY --from=builder /app/build /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf 

# nginx.conf はデフォルトで /etc/nginx/conf.d/default.conf を読み込むため、
# 今回は react-frontend 用の nginx.conf は用意しない。
# 必要に応じて、COPY ./nginx.conf /etc/nginx/conf.d/default.conf 

EXPOSE 80  
#デフォルトで80だが、念の為

# デフォルトの nginx コマンドで起動するため、CMD は不要