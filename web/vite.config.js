import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';
import path from "path";
export default defineConfig({
    plugins: [react()],
    resolve: {
        alias: {
            "@": path.resolve(__dirname, "./src"),
        },
    },
    server: {
        proxy: {
            '/graph.v1.GraphService': {
                target: 'http://localhost:8088',
                changeOrigin: true
            }
        }
    },
    build: {
        rollupOptions: {
            output: {
                onlyExplicitManualChunks: true,
                manualChunks: function (id) {
                    var module = id.replace(/\\/g, '/');
                    if (module.includes('commonjsHelpers') || /\/node_modules\/(react|react-dom|scheduler)\//.test(module))
                        return 'vendor-react';
                    if (/\/node_modules\/(sigma|graphology[^/]*|@react-sigma\/[^/]+)\//.test(module))
                        return 'vendor-graph';
                    if (/\/node_modules\/(@bufbuild|@connectrpc)\//.test(module))
                        return 'vendor-proto';
                    if (/\/node_modules\/react-markdown\//.test(module))
                        return 'vendor-markdown';
                },
            },
        },
    },
    test: {
        environment: 'jsdom',
        setupFiles: ['./src/setupTests.ts'],
        globals: true,
        exclude: ['**/node_modules/**', '**/tests/e2e/**']
    }
});
