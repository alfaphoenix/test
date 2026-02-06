package main

import "errors"

// errMissingBotToken возвращается при отсутствии токена бота.
var errMissingBotToken = errors.New("BOT_TOKEN is not set")

// errInvalidUserID используется при неверном идентификаторе пользователя.
var errInvalidUserID = errors.New("invalid user_id")
