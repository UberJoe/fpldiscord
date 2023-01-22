import os
from PIL import Image, ImageDraw, ImageFont, ImageColor
import requests
from io import BytesIO

class TeamImg:

    image_urls = {
        'pitch': os.path.join(os.path.dirname(__file__), "img/pitch.png"),
        'shirt': "https://draft.premierleague.com/img/shirts/standard/shirt_{0}-66.png"
    }

    team_shirt_numbers = {
        "Arsenal"           :   "3",
        "Aston Villa"       :   "7",
        "Bournemouth"       :   "91",
        "Brentford"         :   "94",
        "Brighton"          :   "36",
        "Chelsea"           :   "8",
        "Crystal Palace"    :   "31",
        "Everton"           :   "11",
        "Fulham"            :   "54",
        "Leeds"             :   "2",
        "Leicester"         :   "13",
        "Liverpool"         :   "14",
        "Man City"          :   "43",
        "Man Utd"           :   "1",
        "Newcastle"         :   "4",
        "Nott'm Forest"     :   "17",
        "Southampton"       :   "20",
        "Spurs"             :   "6",
        "West Ham"          :   "21",
        "Wolves"            :   "39"
    }

    def __init__(self):
        self.session = requests.session()

        self.background = Image.open(self.image_urls["pitch"])

    def main(self, team_df):
        background = self.create_team_image(self.background, team_df)
        return background

    def create_team_image(self, background, team_df):
        goalkeepers_df = team_df.loc[team_df['plural_name'] == 'Goalkeepers']
        defenders_df = team_df.loc[team_df['plural_name'] == 'Defenders']
        midfielders_df = team_df.loc[team_df['plural_name'] == 'Midfielders']
        forwards_df = team_df.loc[team_df['plural_name'] == 'Forwards']

        row_start = background.height // 12
        row_height = background.height // 5

        background = self.add_player_row(background, goalkeepers_df, row_start)
        background = self.add_player_row(background, defenders_df, row_start + row_height)
        background = self.add_player_row(background, midfielders_df, row_start + (row_height*2))
        background = self.add_player_row(background, forwards_df, row_start + (row_height*3))

        return background

    def add_player_row(self, background, players_df, height):
        shirt_width = 66

        gap, margin = self.calculate_gap(shirt_width, background.width, len(players_df.index))
        player_place = margin
        for row in players_df.itertuples():
            if row.plural_name == "Goalkeepers":
                team_num = self.team_shirt_numbers[row.team_name] + "_1"
            else:
                team_num = self.team_shirt_numbers[row.team_name]
                
            url = self.image_urls['shirt']
            r = self.session.get(url.format(team_num))
            b_image = BytesIO(r.content)
            shirt = Image.open(b_image)
            
            offset = (player_place, height)
            background.paste(shirt, offset, shirt)

            w, h = shirt.width + shirt.width // 3 * 2, shirt.height // 4
            textoverlay = self.generate_player_text((w,h), os.getenv('IMG_FONT'), 12, (55,0,60), "White", row.web_name)
            textoffset = (player_place - (shirt.width // 4), height + shirt.height)
            background.paste(textoverlay, textoffset, textoverlay)

            player_place += gap + shirt.width

        return background

    def generate_player_text(self, size, fontname, fontsize, bg, fg, text):
        W, H = size
        canvas = Image.new('RGBA', size, bg)

        draw = ImageDraw.Draw(canvas)
        myfont = ImageFont.truetype(fontname, fontsize)
        
        w, h = draw.textsize(text, font=myfont)

        draw.text(((W-w)/2,(H-h)/2), text, fg, font=myfont)

        return canvas

    def calculate_gap(self, shirt_w, pitch_w, num_players):
        gap = (shirt_w * 3) // 2
        player_row = ((shirt_w * num_players) + (gap * (num_players-1)))
        margin = (pitch_w - player_row) // 2

        return gap, margin