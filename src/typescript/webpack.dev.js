import { merge } from "webpack-merge";
import commonConfig from "./webpack.common.js";

export default merge(commonConfig, {
    devtool: "inline-source-map",
    mode: "development",
});