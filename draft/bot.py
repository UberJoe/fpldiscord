import discord
import os
from discord.ext import commands
from draft import utils
import pandas as pd

description = 'Bot for Coq Au Ian'
intents = discord.Intents.all()
bot = commands.Bot(command_prefix='!', description=description, intents=intents)

# import config as cnf # uncomment if in dev env

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

@bot.group(name='fixtures', invoke_without_command=True)
async def fixtures(ctx):
    matches_df = utils.get_fixtures()
    gw_df = matches_df.loc[matches_df['match'] == utils.current_gw()]

    for row in gw_df.itertuples():
        await ctx.send(row.home_player + " vs " + row.away_player)

@fixtures.command(name='gw')
async def gw_subcommand(ctx, week: int):
    matches_df = utils.get_fixtures()
    gw_df = matches_df.loc[matches_df['match'] == week]

    for row in gw_df.itertuples():
        await ctx.send(row.home_player + " vs " + row.away_player)

@fixtures.command(name='team')
async def team_subcommand(ctx, team: str):
    matches_df = utils.get_fixtures()
    next5_df = matches_df.loc[matches_df['match'] < (utils.current_gw() + 5)]
    team_df = next5_df.loc[(next5_df['home_player'].str.lower() == team.lower()) | (next5_df['away_player'].str.lower() == team.lower())]

    for row in team_df.itertuples():
        await ctx.send(row.home_player + " vs " + row.away_player)

@bot.command()
async def dave(ctx):
    await ctx.send("fuck you Dave")

bot.run(os.environ['token'])