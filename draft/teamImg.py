from PIL import Image, ImageDraw, ImageFont, ImageColor
from cairosvg import svg2png
import requests
from io import BytesIO

class TeamImg:

    image_urls = {
        'pitch': 'https://draft.premierleague.com/static/media/pitch-default.dab51b01.svg',
        'shirt': 'https://draft.premierleague.com/img/shirts/standard/shirt_6-66.png'
    }

    def __init__(self):
        self.session = requests.session()
        pitch_svg = self.session.get(self.image_urls['pitch'])

        pitch_png = BytesIO()
        svg2png(bytestring=pitch_svg.content, write_to=pitch_png)
        self.background = Image.open(pitch_png)


        shirt_png = self.session.get(self.image_urls['shirt'])
        self.shirt = Image.open(BytesIO(shirt_png.content))

    def main(self, team_df):
        background = self.create_team_image(self.background, self.shirt, team_df)
        return background

    def create_team_image(self, background, overlay, team_df):
        goalkeepers_df = team_df.loc[team_df['plural_name'] == 'Goalkeepers']
        defenders_df = team_df.loc[team_df['plural_name'] == 'Defenders']
        midfielders_df = team_df.loc[team_df['plural_name'] == 'Midfielders']
        forwards_df = team_df.loc[team_df['plural_name'] == 'Forwards']

        row_start = background.height // 12
        row_height = background.height // 5

        background = self.add_player_row(background, overlay, goalkeepers_df, row_start)
        background = self.add_player_row(background, overlay, defenders_df, row_start + row_height)
        background = self.add_player_row(background, overlay, midfielders_df, row_start + (row_height*2))
        background = self.add_player_row(background, overlay, forwards_df, row_start + (row_height*3))

        return background

    def add_player_row(self, background, shirt, players_df, height):
        gap, margin = self.calculate_gap(shirt.width, background.width, len(players_df.index))
        player_place = margin
        for row in players_df.itertuples():
            offset = (player_place, height)
            background.paste(shirt, offset, shirt)

            w, h = shirt.width + shirt.width // 3 * 2, shirt.height // 4
            textoverlay = self.generate_player_text((w,h), '/app/.fonts/fonts/Helvetica-Bold.ttf', 10, (55,0,60), "White", row.web_name)
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