import { defineConfig } from "vite";
import react from "@vitejs/plugin-react-swc";
import tailwindcss from "@tailwindcss/vite";
import path from "path";

// https://vite.dev/config/
export default defineConfig({
    plugins: [react(), tailwindcss()],
    server: {
        host: "127.0.0.1",
        allowedHosts: process.env.NGROK_DOMAIN ? [process.env.NGROK_DOMAIN] : [],
        proxy: {
            "/api": "http://127.0.0.1:8080",
            "/telegram": "http://127.0.0.1:8080",
            "/healthz": "http://127.0.0.1:8080",
        },
    },
    resolve: {
        alias: {
            "@": path.resolve(__dirname, "./src"),
        },
    },
});
