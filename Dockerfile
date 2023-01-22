FROM python:3.10
WORKDIR /bot
COPY requirements.txt /bot/
RUN pip install -r requirements.txt
COPY . /bot
RUN mkdir -p /usr/share/fonts/truetype/
RUN install -m644 /bot/fonts/Helvetica-Bold.ttf /usr/share/fonts/truetype/
CMD python draft/bot.py