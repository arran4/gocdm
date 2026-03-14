Wait, `LaunchXSession` drops privileges *before* calling `LaunchXSessionFn`. So it runs as the user.
Wait! What if we don't use `-login`? "especially if `gocdm` is running with higher privileges before dropping them." Wait, if `-login` isn't used, `gocdm` just runs. If it runs as root, it can overwrite anything. But the config file is read from the user running it.
Anyway, how to "do a better job at detecting this especially reducing false positives and negatives"?
A false positive with `strings.Contains(..., "..")` is a file or directory named `..something` or `something..`. Because `..` is just two dots. For example, `my..file.log` contains `..` but is not a path traversal.
To reduce false positives, we should check for path components that are exactly `..`. We can use `strings.Split(filepath.ToSlash(startXLog), "/")` and check if any component is `..`.
Also, to prevent path traversal in absolute paths without false negatives, we should resolve the path and verify it. BUT `filepath.Clean` doesn't retain the fact that there was a `..` that navigated up.
If we split the original path by `/`, and check if any segment is `..`, we accurately identify path traversal tokens without flagging `my..file`.

```go
func hasPathTraversal(p string) bool {
    parts := strings.Split(filepath.ToSlash(p), "/")
    for _, part := range parts {
        if part == ".." {
            return true
        }
    }
    return false
}
```
This is much better and has zero false positives for filenames containing `..` like `my..file`.
Is this sufficient for false negatives? What about Windows? `filepath.ToSlash` handles `\` on Windows. What about URL-encoded `..`? Go's `os.OpenFile` doesn't decode URL encoding.
So `hasPathTraversal` is exactly what we need.

Let's modify `x11/x11.go` to use this logic instead.
