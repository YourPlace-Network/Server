import { merge } from "webpack-merge";
import commonConfig from "./webpack.common.js";
import TerserPlugin from "terser-webpack-plugin";

export default merge(commonConfig, {
    mode: "production",
    output: {
        filename: "[name].[contenthash:8].js",
        chunkFilename: "[name].[contenthash:8].chunk.js",
    },
    optimization: {
        moduleIds: 'deterministic',
        runtimeChunk: 'single',
        minimize: true,
        minimizer: [
            new TerserPlugin({
                terserOptions: {
                    compress: {
                        drop_console: true,
                        drop_debugger: true,
                        pure_funcs: ['console.log', 'console.info', 'console.debug'],
                        passes: 2,
                    },
                    mangle: {
                        safari10: true,
                    },
                    format: {
                        comments: false,
                    },
                },
                extractComments: false,
                parallel: true,
            }),
        ],
        splitChunks: {
            chunks: 'all',
            maxInitialRequests: 25,
            minSize: 20000,
            cacheGroups: {
                tinymce: {
                    test: /[\\/]node_modules[\\/]tinymce[\\/]/,
                    name: 'tinymce',
                    priority: 30,
                    reuseExistingChunk: true,
                },
                vendor: {
                    test: /[\\/]node_modules[\\/]/,
                    name(module) {
                        const packageName = module.context.match(/[\\/]node_modules[\\/](.*?)([\\/]|$)/)[1];
                        return `vendor.${packageName.replace('@', '')}`;
                    },
                    priority: 20,
                },
                common: {
                    minChunks: 2,
                    priority: 10,
                    reuseExistingChunk: true,
                    enforce: true,
                },
            },
        },
    },
});