class DUtils:
    
    def format_response_df(df): 
        response = ""
        for row in df.itertuples():
            response += row.home_player + " vs " + row.away_player + "\n"