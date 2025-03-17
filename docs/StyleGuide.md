# Coding Style Guide

## Introduction

Programming is as much about communicating with other developers as it is about instructing computers. This style guide establishes the shared language we use to collaborate effectively on our codebase.

Good code isn't just functional—it's readable, maintainable, and consistent. By following these guidelines, we ensure that our code remains accessible to new team members, easier to debug, and simpler to extend as our project evolves.

This style guide represents our collective experience and best practices. It covers formatting conventions, naming standards, architecture patterns, and programming idioms specific to our technology stack. While some rules may seem arbitrary, consistency itself has value—when code follows predictable patterns, developers can focus on solving problems rather than deciphering style choices.

Our guidelines are not meant to restrict creativity or enforce rigid conformity. Instead, they provide a framework that enables us to write clean, efficient code that stands the test of time. There will always be exceptions where breaking a rule makes sense—in those cases, document your reasoning clearly.

Remember that this is a living document. As technologies evolve, and we learn from experience, we'll refine these guidelines together. Your feedback and suggestions for improvement are always welcome.

Let's build code that we're proud to write and others are happy to read.

### Back-End Go Code

- Use tabs for indentation instead of spaces. This is the standard for Go code.
- When naming variables, functions, types and packages, use `camelCase`.
- When naming variables, functions, types and packages, use a descriptive name and avoid generic words like "type, data, value, array" unless it's necessary to describe the purpose of the thing. For example, use `avatarURL` instead of just `url`. The name should precisely describe the purpose of the thing with no ambiguity. Verbose names are better.
- When naming variables, functions, types and packages, don't abbreviate words. For example, use `numberOfUsers` instead of `numUsers`.
- Don't create acronyms and only use industry standard ones if necessary. Things like "SQL", "URL", "HTTP", "API", "JSON", etc. are fine.
- Reduce third party dependencies and modules. If possible, and while considering time constraints, write your own code to perform the necessary actions.
- Do not use GOTOs and Go labels for control flow. There is no legitimate use-case for this design pattern, and it makes the code much harder to read and understand. Instead, use functions and control flow statements like if, for, and switch.
- Avoid using inline \"if assignment statements\" such as `if err := doStuff(); err != nil {`. Instead, put the `err := doStuff()` on its own line, then check `if err != nil` afterward. This inline style might be shorter, but it's harder to read and is prone to mistakes.
- Avoid wrapping lines of code. Long lines are fine if necessary. This makes the code easier to read and understand and helps the IDE syntax parser.
- SQL queries should be written on one line so that the IDE can understand it and provide syntax highlighting. You can always copy/paste long SQL queries into a SQL editor for testing outside the main code base.
- SQL queries should capitalize SQL reserved words like `SELECT`, `FROM`, `INSERT`, `WHERE`, etc. This makes the query easier to read and understand what is the language, versus what is a variable that we define.
- Don't concatenate strings in SQL which can lead to injection issues. Always use prepared statements and parameterized queries. These already exist in the SQL wrappers we have in the code base.
- Use the `core.LogDebug` / `core.LogInfo` / `core.LogError` / etc. functions for logging. Avoid using the Go language `fmt.Println` or similar for logging.
- If your function needs to return an error, use the `core.LogErrorReturn` function as it will send the logs to our standard logging pipeline and also return the error type you need.
- Always use double-quotes `"` for strings unless absolutely necessary to use single-quotes `'` or backticks `` ` ``.
- Avoid passing around `interface{}` types. This makes the code harder to read and understand. Instead, convert it to a specific type as soon as possible or create a new type if necessary.
- Scrutinize the use of semicolons `;` in Go. There are few, if any, necessary use-cases for them.
- There are very few scenarios where we should be using `panic` or `os.Exit` in Go. If you need to halt the program in an emergency, use `host.Shutdown` with an exit code instead. This ensures the program shuts down gracefully.
- When returning JSON in Gin-Gonic, prefer to use `c.SecureJSON` instead of `c.JSON`. This will help ensure that the JSON is secure and can't be hijacked by a malicious user.
- Don't surface error messages or user-controlled data in the HTTP response or in any response headers, unless absolutely necessary and only after it's been sanitized and validated.
- Avoid putting user-controlled data into any log messages. This can lead to log injection attacks and can expose sensitive data or exhaust resources.
- You may put system error messages like `err.Error()` into logs, but try to avoid it if possible. Instead, log a more descriptive message that explains the error and what the system is doing to recover from it.
- Use unique error messages so that you can easily search for the breaking code. This will help you find the error in the logs and in the code base.
- Use proper capitalization and punctuation in your error messages as they may be exposed to the user in the UI. Error messages are designed for humans to read.
- Avoid multiple inline function calling in Go. Instead, call the first function on its own line. Then the next one on its own line. This makes the code easier to read and understand.
- Only use crypto functions in the `cryptography.go` file. This ensures that we have a consistent and easily auditable way of handling security and encryption in the code base.
- Avoid using reflection in Go. It's slow, hard to read, and hard to debug. If you need to use reflection, consider if there is a better way to do it.
- Most (but not all) error scenarios require bailing out of the current execution context. If necessary, be sure to include `return`, `break`, or `continue` after the error is logged.
- Try to not use `time.Sleep` or equivalent in Go. There are some scenarios where it's necessary, but it's not often and usually only in the context of waiting for the OS to perform some task.

### Back-End Shell Code

### Front-End Typescript Code

### Front-End HTML Code

### Front-End SCSS Code