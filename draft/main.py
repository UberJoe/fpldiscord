import discord 
from dotenv import load_dotenv
import os

dotenv_path = 'config.env'
load_dotenv(dotenv_path=dotenv_path)

if os.getenv('DEBUG_GUILDS') is not None:
    client = discord.Bot(debug_guilds=[os.getenv('DEBUG_GUILDS')])
else:
    client = discord.Bot()

for f in os.listdir("./draft/cogs"):
    if f.endswith(".py"):
        client.load_extension("cogs." + f[:-3])

client.run(os.getenv('TOKEN'))