import discord
from discord.ext import commands
from draft import utils
import pandas as pd
import config as cnf
from boto.s3.connection import S3Connection

description = 'Bot for Coq Au Ian'
intents = discord.Intents.all()
bot = commands.Bot(command_prefix='!', description=description, intents=intents)

@bot.command()
async def refresh_data(ctx):
    utils.get_json()
    await ctx.send("Done, I hope")

@bot.command()
async def owner(ctx, *, player: str):
    elements_df = utils.get_data('elements')
    element_status_df = utils.get_data('element_status')
    players_df = pd.merge(elements_df, element_status_df, how="outer", left_on="id", right_on="element")
    
    results_df = players_df.loc[players_df['web_name'].str.lower() == player.lower()]
    if results_df.empty:
        await ctx.send("Player not found")
        return

    for row in results_df.itertuples():
        if row.owner is not None:
            await ctx.send(row.first_name + ' ' + row.second_name + ' is owned by: ' + row.owner)
        else:
            await ctx.send(row.first_name + ' ' + row.second_name + ' is a free agent!')

@bot.command()
async def dave(ctx):
    await ctx.send("fuck you Dave")

bot.run(S3Connection(os.environ['token']))