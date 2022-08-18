import io
from fplutils import Utils
from teamImg import TeamImg
import discord
from discord import Option
import os
from dotenv import load_dotenv
import pandas as pd

dotenv_path = 'config.env'
load_dotenv(dotenv_path=dotenv_path)

if os.getenv('DEBUG_GUILDS') is not None:
    bot = discord.Bot(debug_guilds=[os.getenv('DEBUG_GUILDS')])
else:
    bot = discord.Bot()

img = TeamImg()
u = Utils()

fixtures = bot.create_group("fixtures", "retrieve fixtures")

@bot.command(description="Finds the owner for the specified player")
async def owner(ctx, *, player_name: discord.Option(str, description="Player's name")):
    player_df = u.get_player_attr(player_name, "team_x")

    if player_df.empty:
        await ctx.respond("Player not found")
        return

    response = ''
    for row in player_df.itertuples():
        if (type(row.team_x) == str):
            response += row.first_name + ' ' + row.second_name + ' is owned by: ' + row.team_x + '\n'
        else:
            response += row.first_name + ' ' + row.second_name + ' is a free agent!\n'
    await ctx.respond(response)

@fixtures.command(description="Responds with the H2H fixtures for the coming GW")
async def this_week(ctx):
    matches_df = u.get_fixtures()
    gw_df = matches_df.loc[matches_df['match'] == u.current_gw()]

    response = ""
    for row in gw_df.itertuples():
        response += row.home_player + " vs " + row.away_player + "\n"
    await ctx.respond(response)

@fixtures.command(name='gw', description="Responds with the H2H fixtures for the specified GW")
async def gw_subcommand(ctx, gameweek: discord.Option(int, description="Gameweek number")):
    matches_df = u.get_fixtures()
    gw_df = matches_df.loc[matches_df['match'] == gameweek]

    response = ""
    for row in gw_df.itertuples():
        response += row.home_player + " vs " + row.away_player + "\n"
    await ctx.respond(response)

@fixtures.command(name='team', description="Responds with the next 5 H2H fixtures for the specified team")
async def team_subcommand(ctx, team: discord.Option(str, description="Team owner's first name")):
    matches_df = u.get_fixtures()
    next5_df = matches_df.loc[matches_df['match'] < (u.current_gw() + 5)]
    team_df = next5_df.loc[(next5_df['home_player'].str.lower() == team.lower()) | (next5_df['away_player'].str.lower() == team.lower())]

    response = ""
    for row in team_df.itertuples():
        response += row.home_player + " vs " + row.away_player + "\n"
    await ctx.respond(response)

@bot.command(description="Responds with the team of the specified owner as a list")
async def teamlist(ctx, owner: discord.Option(str, description="Team owner's first name")):
    team_df = u.get_team(owner)

    team_gk_df = team_df.loc[team_df['plural_name'] == 'Goalkeepers']
    team_def_df = team_df.loc[team_df['plural_name'] == 'Defenders']
    team_mid_df = team_df.loc[team_df['plural_name'] == 'Midfielders']
    team_att_df = team_df.loc[team_df['plural_name'] == 'Forwards']

    response = ""
    for row in team_gk_df.itertuples():
        response += "**" + row.web_name + "**" + " (" + row.short_name + ")\n"
    response += "\n"
    for row in team_def_df.itertuples():
        response += "**" + row.web_name + "**" + " (" + row.short_name + ")\n"
    response += "\n"
    for row in team_mid_df.itertuples():
        response += "**" + row.web_name + "**" + " (" + row.short_name + ")\n"
    response += "\n"
    for row in team_att_df.itertuples():
        response += "**" + row.web_name + "**" + " (" + row.short_name + ")\n"
    await ctx.respond(response)

@bot.command(description="Responds with this week's waivers")
async def waivers(ctx, gameweek: discord.Option(int, description="GW to show waivers of (0 for all waivers)", max_value=38, min_value=0) = u.current_gw()):
    this_gw = u.current_gw()
    if gameweek > this_gw:
        gameweek = this_gw
    transactions_df = u.get_transactions(gameweek)
    if transactions_df.empty:
        await ctx.respond("Couldn't find any waivers for GW" + str(gameweek))
        return

    response = "**Waivers in GW" + str(gameweek) + ":**\n"
    for row in transactions_df.itertuples():
        response += row.short_name + ":  " + row.element_out + "  ->  " + row.element_in
        if row.result == 'a':
            response += " :white_check_mark:\n"
        else:
            response += " :x:\n"
    await ctx.respond(response)

@bot.command(description="Responds with a message for whenever Dave pipes up")
async def dave(ctx):
    await ctx.respond("fuck you Dave")

@bot.command(description="Responds with an image of the team owned by the specified owner")
async def team(ctx, owner: discord.Option(str, description="Team owner's first name")):
    team_df = u.get_team(owner)
    team_image = img.main(team_df)
    with io.BytesIO() as image_binary:
        team_image.save(image_binary, 'PNG')
        image_binary.seek(0)
        await ctx.respond(owner + "'s team", file=discord.File(fp=image_binary, filename='team_image.png'))

bot.run(os.getenv('TOKEN'))