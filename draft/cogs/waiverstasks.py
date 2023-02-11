import os
import datetime
from datetime import timedelta
import asyncio
from dotenv import load_dotenv
import discord
from discord.ext import tasks, commands
from fplutils import Utils
from apscheduler.schedulers.asyncio import AsyncIOScheduler
from apscheduler.triggers.cron import CronTrigger
import logging

dotenv_path = 'config.env'
load_dotenv(dotenv_path=dotenv_path)

class WaiversTasks(commands.Cog):

    def __init__(self, client):
        self.client = client
        self.notifications_channel = os.getenv('NOTIFICATION_CHANNEL')
        self.u = Utils()
        self.waiver_reminder.start()

    @tasks.loop(hours=24)
    async def waiver_reminder(self):
        waiver_time = self.u.get_waiver_time()
        waiver_time = "2023-02-11 08:47:00"
        waiver_time = datetime.datetime.strptime(waiver_time, '%Y-%m-%d %H:%M:%S')

        if (waiver_time.day == datetime.datetime.today().day):
            today_msg = "```" + "Waivers are happening today at: " + str(waiver_time) + "```"
            await self.notify(today_msg)

            await asyncio.sleep(await self.seconds_until((waiver_time.hour - 1) ,waiver_time.minute))  # Will stay here until your clock says 11:58
            hour_msg = "```" + "Waivers are in ONE HOUR at: " + str(waiver_time) + "```"
            await self.notify(hour_msg)
            await asyncio.sleep(60)  # Practical solution to ensure that the print isn't spammed as long as it is 11:58

    @waiver_reminder.before_loop
    async def wait_until(self):
        await asyncio.sleep(await self.seconds_until(7,43))

    async def notify(self, msg):
        channel = await self.client.fetch_channel(self.notifications_channel)
        await channel.send(msg)

    async def seconds_until(self, hours, minutes):
        given_time = datetime.time(hours, minutes)
        now = datetime.datetime.now()
        future_exec = datetime.datetime.combine(now, given_time)
        if (future_exec - now).days < 0:  # If we are past the execution, it will take place tomorrow
            future_exec = datetime.datetime.combine(now + timedelta(days=1), given_time) # days always >= 0

        return (future_exec - now).total_seconds()

def setup(client):
    client.add_cog(WaiversTasks(client))
