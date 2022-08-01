import discord
import os
from discord.ext import commands
from draft import utils
import pandas as pd

import config as cnf

description = 'Bot for Coq Au Ian'
intents = discord.Intents.all()
bot = commands.Bot(command_prefix='!', description=description, intents=intents)

@bot.command()
async def refresh_data(ctx):
    utils.get_json()
    await ctx.send("Done, I hope")

@bot.command()
async def owner(ctx, *, player: str):
    players_df = utils.get_team_players()
    
    player_df = players_df.loc[players_df['web_name'].str.lower() == player.lower()]
    if player_df.empty:
        await ctx.send("Player not found")
        return

    for row in player_df.itertuples():
        if (type(row.team_x) == str):
            await ctx.send(row.first_name + ' ' + row.second_name + ' is owned by: ' + row.team_x)
        else:
            await ctx.send(row.first_name + ' ' + row.second_name + ' is a free agent!')

@bot.command()
async def dave(ctx):
    await ctx.send("fuck you Dave")

bot.run(os.environ['token'])