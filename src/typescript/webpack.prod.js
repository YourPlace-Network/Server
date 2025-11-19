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
            cacheGroups: {
                vendor: {
                    test: /[\\/]node_modules[\\/]/,
                    name: 'vendors',
                    priority: 20,
                    reuseExistingChunk: true,
                },
                common: {
                    minChunks: 2,
                    name: 'common',
                    priority: 10,
                    reuseExistingChunk: true,
                    enforce: true,
                },
            },
        },
    },
});