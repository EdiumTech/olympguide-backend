import os

REDIS_HOST = os.getenv("REDIS_HOST")
REDIS_PORT = int(os.getenv("REDIS_PORT"))
REDIS_PASSWORD = os.getenv("REDIS_PASSWORD")
EMAIL_CODE_TOPIC = "auth.email.code"
PASSWORD_TOPIC = "auth.email.password"

REDIS_TOPICS = [EMAIL_CODE_TOPIC, PASSWORD_TOPIC]

SMTP_SERVER = os.getenv("SMTP_SERVER", "smtp.mail.ru")
SMTP_PORT = int(os.getenv("SMTP_PORT", "587"))
SMTP_USERNAME = os.getenv("SMTP_USERNAME")
SMTP_PASSWORD = os.getenv("SMTP_PASSWORD")
