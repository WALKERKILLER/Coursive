import { readFileSync } from "fs";
export const read = () => readFileSync("/etc/passwd", "utf8");
