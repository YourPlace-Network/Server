import path from "path";
import { fileURLToPath } from "url";
import webpack from "webpack";
import CopyPlugin from "copy-webpack-plugin";
import MiniCssExtractPlugin from "mini-css-extract-plugin";
import { WebpackManifestPlugin } from "webpack-manifest-plugin";

// Get __dirname equivalent in ESM
const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

export default {
    context: path.resolve(__dirname, ""),
    entry: {
        faq: "./pages/faq.ts",
        files: "./pages/files.ts",
        home: "./pages/home.ts",
        login: "./pages/login.ts",
        logout: "./pages/logout.ts",
        mentalHealth: "./pages/mentalHealth.ts",
        notFound: "./pages/notFound.ts",
        notifications: "./pages/notifications.ts",
        post: "./pages/post.ts",
        profile: "./pages/profile.ts",
        settings: "./pages/settings.ts",
        setup: "./pages/setup.ts",
        spotifyCallback: "./pages/spotifyCallback.ts",
        test: "./pages/test.ts",
        tinymce: "../scss/tinymce.scss",
    },
    mode: "production",
    module: {
        rules: [{
            test: /node_modules\/@avmkit\/siwa/,
            resolve: {
                fullySpecified: false,
            },
        },{
            test: /tinymce\/skins\/.*\.(css|svg|ttf|woff|woff2)$/,
            type: "asset/resource",
            generator: {
                filename: (pathData) => {
                    const path = pathData.filename.replace(/.*\/node_modules\/tinymce\//, "");
                    return `../${path}`;
                }
            }
        },{
            test: /\.(ttf|eot|svg|woff|woff2)$/,
            type: "asset/resource",
            generator: {
                filename: "../fonts/[name][ext]"
            }
        },{
            test: /\.tsx?$/,
            use: [{
                loader: "ts-loader",
                options: {
                    configFile: path.resolve(__dirname, "./tsconfig.json"),
                }
            }],
            include: path.resolve(__dirname, "."),
            exclude: /node_modules/,
        },{
            test: /\.css$/,
            include: path.resolve(__dirname, "../../node_modules/flatpickr"),
            use: ["style-loader", "css-loader"],
        },{
            test: /tinymce\.scss$/,
            include: path.resolve(__dirname, "../scss"),
            use: [{
                loader: MiniCssExtractPlugin.loader
            },{
                loader: "css-loader",
                options: { sourceMap: false }
            },{
                loader: "postcss-loader",
                options: { postcssOptions: { plugins: [["autoprefixer", {}]] } }
            },{
                loader: "sass-loader",
                options: { sourceMap: false, sassOptions: { outputStyle: "compressed", quietDeps: true, silenceDeprecations: ["color-functions", "global-builtin", "import"] } }
            }]
        },{
            test: /\.(sass|scss|css)$/,
            include: path.resolve(__dirname, "../scss"),
            exclude: [/node_modules/, /tinymce\.scss$/],
            use: [{
                loader: "style-loader"  // Adds CSS to the DOM by injecting a `<style>` tag
            },{
                loader: "css-loader",  // Interprets `@import` and `url()` like `import/require()` and will resolve them
                options: {
                    sourceMap: false,
                }
            },{
                loader: "postcss-loader",  // Loader for webpack to process CSS with PostCSS
                options: {
                    postcssOptions: {
                        plugins: [["autoprefixer", {}]]
                    }
                }
            },{
                loader: "sass-loader",  // Loads a SASS/SCSS file and compiles it to CSS
                options: {
                    sourceMap: false,
                    sassOptions: {
                        outputStyle: "compressed",
                        quietDeps: true,
                        silenceDeprecations: ["color-functions", "global-builtin", "import"],
                    }
                }
            }]
        }]
    },
    output: {
        filename: "[name].js",
        chunkFilename: "[name].chunk.js",
        path: path.resolve(__dirname, "../www/js/"),
        clean: true,
    },
    resolve: {
        extensions: [".tsx", ".ts", ".jsx", ".js"],
        modules: [path.resolve(__dirname, "."), "node_modules"],
        fallback: {
            "crypto": "crypto-browserify",
            "stream": "stream-browserify",
            "buffer": "buffer/",
            "util": "util/",
            "process": false,
            "path": "path-browserify",
        },
        alias: {
            "apg-js/src/apg-api/api": "apg-js/src/apg-api/api.js",
            "apg-js/src/apg-lib/node-exports": "apg-js/src/apg-lib/node-exports.js",
            process: "process/browser",
        }
    },
    plugins: [
        new webpack.IgnorePlugin({
            resourceRegExp: /^@react-native-async-storage\/async-storage$/,
        }),
        new webpack.ProvidePlugin({
            Buffer: ["buffer", "Buffer"],
        }),
        new webpack.ProvidePlugin({
            process: "process/browser",
        }),
        new CopyPlugin({
            patterns: [{
                from: path.resolve(__dirname, "../../node_modules/tinymce"),
                to: path.resolve(__dirname, "../www/tinymce"),
            }],
        }),
        new MiniCssExtractPlugin({
            filename: "../css/[name].css",
        }),
        new WebpackManifestPlugin({
            fileName: "../manifest.json",
            publicPath: "/static/js/",
        })
    ],
    experiments: {
        topLevelAwait: true,
    },
    ignoreWarnings: [{
        message: /print-color-adjust/,
    }],
    target: "web",
};